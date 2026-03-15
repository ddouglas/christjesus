package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"christjesus/pkg/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *Service) handleGetProfileNeedEditDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	documents, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch documents")
		s.internalServerError(w)
		return
	}

	options := documentTypeOptions()
	optionViews := make([]any, len(options))
	for i, option := range options {
		optionViews[i] = option
	}

	data := &types.NeedDocumentsPageData{
		BasePageData:        types.BasePageData{Title: "Edit Need Documents"},
		ID:                  needID,
		Documents:           documents,
		HasDocuments:        len(documents) > 0,
		Notice:              r.URL.Query().Get("notice"),
		Error:               r.URL.Query().Get("error"),
		DocumentTypeOptions: optionViews,
		MetadataAction:      s.route(RouteProfileNeedEditMeta, map[string]string{"needID": needID}),
		UploadAction:        s.route(RouteProfileNeedEditUpload, map[string]string{"needID": needID}),
		ContinueAction:      s.route(RouteProfileNeedEditDocs, map[string]string{"needID": needID}),
		BackHref:            s.route(RouteProfileNeedEditStory, map[string]string{"needID": needID}),
		DeleteActions:       s.needDocumentDeleteActions(RouteProfileNeedEditDelete, needID, needDocumentIDs(documents)),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.documents", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need documents edit page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditDocumentsUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found")
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "User authentication error. Please log in again.")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Invalid form submission.")
		return
	}

	files := r.MultipartForm.File["documents"]
	if len(files) == 0 {
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Please choose at least one file to upload")
		return
	}

	uploadedCount := 0
	failedCount := 0
	failedFiles := make([]string, 0)

	for _, fileHeader := range files {
		err := s.handleFile(ctx, needID, userID, fileHeader)
		if err != nil {
			s.logger.WithError(err).Error("failed to handle uploaded file")
			failedCount++
			failedFiles = append(failedFiles, fileHeader.Filename)
		} else {
			uploadedCount++
		}
	}

	if uploadedCount == 0 {
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Failed to upload files. Please try again.")
		return
	}

	if failedCount > 0 {
		summary := fmt.Sprintf("Uploaded %d file(s). %d file(s) could not be uploaded", uploadedCount, failedCount)
		if len(failedFiles) > 0 {
			preview := failedFiles
			if len(preview) > 3 {
				preview = preview[:3]
				summary = fmt.Sprintf("%s (%s, +%d more).", summary, strings.Join(preview, ", "), len(failedFiles)-3)
			} else {
				summary = fmt.Sprintf("%s (%s).", summary, strings.Join(preview, ", "))
			}
		} else {
			summary = fmt.Sprintf("%s.", summary)
		}
		s.redirectProfileNeedEditDocsWithNotice(w, r, needID, summary)
		return
	}

	s.redirectProfileNeedEditDocsWithNotice(w, r, needID, fmt.Sprintf("Successfully uploaded %d file(s).", uploadedCount))
}

func (s *Service) handlePostProfileNeedEditDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse documents continue form")
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Invalid form submission.")
		return
	}

	skipDocuments := r.FormValue("skipDocuments") == "on"
	documents, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch documents")
		s.internalServerError(w)
		return
	}

	if len(documents) == 0 && !skipDocuments {
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Upload at least one document or confirm you want to continue without documents.")
		return
	}

	need.CurrentStep = types.NeedStepDocuments
	if err := s.needsRepo.UpdateNeed(ctx, need.ID, need); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to update need step")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepDocuments)
	http.Redirect(w, r, s.route(RouteProfileNeedEditReview, map[string]string{"needID": needID}), http.StatusSeeOther)
}

func (s *Service) handlePostProfileNeedEditDocumentMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	documentIDs := r.Form["document_id"]
	fileNames := r.Form["file_name"]
	documentTypes := r.Form["document_type"]

	if len(documentIDs) == 0 {
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "No documents submitted for update")
		return
	}
	if len(documentIDs) != len(fileNames) || len(documentIDs) != len(documentTypes) {
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Form submission error. Please try again.")
		return
	}

	for i := range documentIDs {
		id := strings.TrimSpace(documentIDs[i])
		name := strings.TrimSpace(fileNames[i])
		dtype := strings.TrimSpace(documentTypes[i])

		if id == "" || name == "" || dtype == "" {
			s.redirectProfileNeedEditDocsWithError(w, r, needID, "Document ID, Name, and Type are required")
			return
		}
		if !isAllowedDocumentType(dtype) {
			s.redirectProfileNeedEditDocsWithError(w, r, needID, fmt.Sprintf("Document type '%s' is not allowed", dtype))
			return
		}

		doc, err := s.documentRepo.DocumentByNeedIDAndID(ctx, needID, id)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", needID).WithField("document_id", id).Error("failed to fetch document for metadata update")
			s.redirectProfileNeedEditDocsWithError(w, r, needID, "Document not found. Please try again.")
			return
		}

		doc.FileName = name
		doc.DocumentType = dtype
		if err := s.documentRepo.UpdateDocument(ctx, doc); err != nil {
			s.logger.WithError(err).WithField("need_id", needID).WithField("document_id", id).Error("failed to update document metadata")
			s.redirectProfileNeedEditDocsWithError(w, r, needID, "Failed to update document metadata. Please try again.")
			return
		}
	}

	s.redirectProfileNeedEditDocsWithNotice(w, r, needID, "Document metadata updated just now.")
}

func (s *Service) handlePostProfileNeedEditDocumentDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	documentID := strings.TrimSpace(r.PathValue("documentID"))

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if documentID == "" {
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Invalid document request.")
		return
	}

	doc, err := s.documentRepo.DocumentByNeedIDAndID(ctx, needID, documentID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).WithField("document_id", documentID).Error("failed to fetch document for delete")
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Document not found.")
		return
	}

	_, err = s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.config.S3BucketName), Key: aws.String(doc.StorageKey)})
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).WithField("document_id", documentID).WithField("storage_key", doc.StorageKey).Error("failed to delete document from S3")
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Could not delete file from storage. Please try again.")
		return
	}

	if err := s.documentRepo.DeleteDocumentByNeedIDAndID(ctx, needID, documentID); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).WithField("document_id", documentID).Error("failed to delete document record")
		s.redirectProfileNeedEditDocsWithError(w, r, needID, "Could not delete document metadata. Please try again.")
		return
	}

	s.redirectProfileNeedEditDocsWithNotice(w, r, needID, "Document removed.")
}

func (s *Service) redirectProfileNeedEditDocsWithError(w http.ResponseWriter, r *http.Request, needID, msg string) {
	q := url.Values{}
	q.Set("error", msg)
	http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedEditDocs, map[string]string{"needID": needID}, q), http.StatusSeeOther)
}

func (s *Service) redirectProfileNeedEditDocsWithNotice(w http.ResponseWriter, r *http.Request, needID, msg string) {
	q := url.Values{}
	q.Set("notice", msg)
	http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedEditDocs, map[string]string{"needID": needID}, q), http.StatusSeeOther)
}
