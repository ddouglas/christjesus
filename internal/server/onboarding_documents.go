package server

import (
	"christjesus/internal/utils"
	"christjesus/pkg/types"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *Service) handleGetOnboardingNeedDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	// Get existing documents
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
		BasePageData:        types.BasePageData{Title: "Upload Documents"},
		ID:                  needID,
		Documents:           documents,
		HasDocuments:        len(documents) > 0,
		Notice:              r.URL.Query().Get("notice"),
		Error:               r.URL.Query().Get("error"),
		DocumentTypeOptions: optionViews,
		MetadataAction:      s.route(RouteOnboardingNeedDocumentsMeta, map[string]string{"needID": needID}),
		UploadAction:        s.route(RouteOnboardingNeedDocumentsUpload, map[string]string{"needID": needID}),
		ContinueAction:      s.route(RouteOnboardingNeedDocuments, map[string]string{"needID": needID}),
		BackHref:            s.route(RouteOnboardingNeedStory, map[string]string{"needID": needID}),
		DeleteActions:       s.needDocumentDeleteActions(RouteOnboardingNeedDocumentDelete, needID, needDocumentIDs(documents)),
	}

	err = s.renderTemplate(w, r, "page.onboarding.need.documents", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need documents page")
		s.internalServerError(w)
		return
	}
}

const (
	maxUploadSizeBytes = 10 << 20 // 10MB
)

func (s *Service) handlePostOnboardingNeedDocumentsUpload(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found")
		s.redirectDocsWithError(w, r, needID, "User authentication error. Please log in again.")
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		s.redirectDocsWithError(w, r, needID, "Invalid form submission.")
		return
	}

	files := r.MultipartForm.File["documents"]
	if len(files) == 0 {
		s.redirectDocsWithError(w, r, needID, "Please choose at least one file to upload")
		return
	}

	uploadedCount := 0
	failedCount := 0
	failedFiles := make([]string, 0)

	for _, fileHeader := range files {
		err = s.handleFile(ctx, needID, userID, fileHeader)
		if err != nil {
			s.logger.WithError(err).Error("failed to handle uploaded file")
			failedCount++
			failedFiles = append(failedFiles, fileHeader.Filename)
		} else {
			uploadedCount++
		}
	}

	if uploadedCount == 0 {
		s.redirectDocsWithError(w, r, needID, "Failed to upload files. Please try again.")
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

		s.redirectDocsWithNotice(w, r, needID, summary)
	} else {
		s.redirectDocsWithNotice(w, r, needID, fmt.Sprintf("Successfully uploaded %d file(s).", uploadedCount))
	}

}

func (s *Service) handleFile(ctx context.Context, needID, userID string, fileHeader *multipart.FileHeader) error {
	if fileHeader.Size <= 0 {
		return utils.ErrorWrapOrNil(fmt.Errorf("file size is zero"), "")
	}
	if fileHeader.Size > maxUploadSizeBytes {
		return utils.ErrorWrapOrNil(fmt.Errorf("file too large"), "")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return utils.ErrorWrapOrNil(err, "failed to open uploaded file")
	}

	defer file.Close()
	ext := filepath.Ext(fileHeader.Filename)

	docID := utils.NanoID()
	storageKey := fmt.Sprintf("needs/%s/%s%s", needID, docID, ext)
	contentType := fileHeader.Header.Get("Content-Type")

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.S3BucketName),
		Key:         aws.String(storageKey),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return utils.ErrorWrapOrNil(err, "failed to upload file to S3")
	}

	// Create document record in database
	doc := &types.NeedDocument{
		ID:            docID,
		NeedID:        needID,
		UserID:        userID,
		DocumentType:  types.DocTypeOther, // Could be enhanced to detect type from filename/form
		FileName:      fileHeader.Filename,
		FileSizeBytes: fileHeader.Size,
		MimeType:      contentType,
		StorageKey:    storageKey,
		UploadedAt:    time.Now(),
	}

	err = s.documentRepo.CreateDocument(ctx, doc)
	if err != nil {
		_, deleteErr := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.config.S3BucketName),
			Key:    aws.String(storageKey),
		})
		if deleteErr != nil {
			s.logger.WithError(deleteErr).WithField("storage_key", storageKey).Warn("failed to clean up uploaded file after DB error")
		}

		return utils.ErrorWrapOrNil(err, "failed to create document record")
	}

	return nil

}

func (s *Service) redirectDocsWithError(w http.ResponseWriter, r *http.Request, needID, msg string) {
	q := url.Values{}
	q.Set("error", msg)
	http.Redirect(w, r, s.routeWithQuery(RouteOnboardingNeedDocuments, map[string]string{"needID": needID}, q), http.StatusSeeOther)
}

func (s *Service) redirectDocsWithNotice(w http.ResponseWriter, r *http.Request, needID, msg string) {
	q := url.Values{}
	q.Set("notice", msg)
	http.Redirect(w, r, s.routeWithQuery(RouteOnboardingNeedDocuments, map[string]string{"needID": needID}, q), http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse documents continue form")
		s.redirectDocsWithError(w, r, needID, "Invalid form submission.")
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
		s.redirectDocsWithError(w, r, needID, "Upload at least one document or confirm you want to continue without documents.")
		return
	}

	need.CurrentStep = types.NeedStepDocuments
	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to update need step")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepDocuments)
	http.Redirect(w, r, s.route(RouteOnboardingNeedReview, map[string]string{"needID": needID}), http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedDocumentMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	err = r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	documentIDs := r.Form["document_id"]
	fileNames := r.Form["file_name"]
	documentTypes := r.Form["document_type"]

	if len(documentIDs) == 0 {
		s.logger.Error("no document IDs provided in form")
		s.redirectDocsWithError(w, r, needID, "No documents submitted for update")
		return
	}

	if len(documentIDs) != len(fileNames) || len(documentIDs) != len(documentTypes) {
		s.logger.Error("mismatched form data lengths")
		s.redirectDocsWithError(w, r, needID, "Form submission error. Please try again.")
		return
	}

	for i := range documentIDs {
		id := strings.TrimSpace(documentIDs[i])
		name := strings.TrimSpace(fileNames[i])
		dtype := strings.TrimSpace(documentTypes[i])

		if id == "" || name == "" || dtype == "" {
			s.redirectDocsWithError(w, r, needID, "Document ID, Name, and Type are required")
			return
		}

		if !isAllowedDocumentType(dtype) {
			s.redirectDocsWithError(w, r, needID, fmt.Sprintf("Document type '%s' is not allowed", dtype))
			return
		}

		doc, err := s.documentRepo.DocumentByNeedIDAndID(ctx, needID, id)
		if err != nil {
			s.logger.WithError(err).
				WithField("need_id", needID).
				WithField("document_id", id).
				Error("failed to fetch document for metadata update")
			s.redirectDocsWithError(w, r, needID, "Document not found. Please try again.")
			return
		}

		doc.FileName = name
		doc.DocumentType = dtype

		err = s.documentRepo.UpdateDocument(ctx, doc)
		if err != nil {
			s.logger.WithError(err).
				WithField("need_id", needID).
				WithField("document_id", id).
				Error("failed to update document metadata")
			s.redirectDocsWithError(w, r, needID, "Failed to update document metadata. Please try again.")
			return
		}
	}

	s.redirectDocsWithNotice(w, r, needID, "Document metadata updated just now.")

}

func (s *Service) handlePostOnboardingNeedDocumentDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")
	documentID := r.PathValue("documentID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	if documentID == "" {
		s.redirectDocsWithError(w, r, needID, "Invalid document request.")
		return
	}

	doc, err := s.documentRepo.DocumentByNeedIDAndID(ctx, needID, documentID)
	if err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			Error("failed to fetch document for delete")
		s.redirectDocsWithError(w, r, needID, "Document not found.")
		return
	}

	_, err = s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.S3BucketName),
		Key:    aws.String(doc.StorageKey),
	})
	if err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			WithField("storage_key", doc.StorageKey).
			Error("failed to delete document from S3")
		s.redirectDocsWithError(w, r, needID, "Could not delete file from storage. Please try again.")
		return
	}

	err = s.documentRepo.DeleteDocumentByNeedIDAndID(ctx, needID, documentID)
	if err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("document_id", documentID).
			Error("failed to delete document record")
		s.redirectDocsWithError(w, r, needID, "Could not delete document metadata. Please try again.")
		return
	}

	s.redirectDocsWithNotice(w, r, needID, "Document removed.")

}

type documentTypeOption struct {
	Value string
	Label string
}

func documentTypeOptions() []documentTypeOption {
	return []documentTypeOption{
		{Value: types.DocTypeID, Label: "ID"},
		{Value: types.DocTypeUtilityBill, Label: "Utility Bill"},
		{Value: types.DocTypeMedicalRecord, Label: "Medical Record"},
		{Value: types.DocTypeIncomeVerification, Label: "Income Verification"},
		{Value: types.DocTypeEvictionNotice, Label: "Eviction Notice"},
		{Value: types.DocTypeOther, Label: "Other"},
	}
}

func allowedDocumentTypes() []string {
	return []string{
		types.DocTypeID,
		types.DocTypeUtilityBill,
		types.DocTypeMedicalRecord,
		types.DocTypeIncomeVerification,
		types.DocTypeEvictionNotice,
		types.DocTypeOther,
	}
}

func isAllowedDocumentType(value string) bool {
	for _, docType := range allowedDocumentTypes() {
		if value == docType {
			return true
		}
	}

	return false
}

func documentTypeLabel(value string) string {
	switch value {
	case types.DocTypeID:
		return "ID"
	case types.DocTypeUtilityBill:
		return "Utility Bill"
	case types.DocTypeMedicalRecord:
		return "Medical Record"
	case types.DocTypeIncomeVerification:
		return "Income Verification"
	case types.DocTypeEvictionNotice:
		return "Eviction Notice"
	case types.DocTypeOther:
		return "Other"
	default:
		return value
	}
}
