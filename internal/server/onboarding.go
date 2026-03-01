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
	s3t "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (s *Service) handleGetOnboarding(w http.ResponseWriter, r *http.Request) {

	var _ = r.Context()

	data := struct {
		Title string
	}{
		Title: "Onboarding",
	}

	err := s.templates.ExecuteTemplate(w, "page.onboarding", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

type onboardingDirector struct {
	Path string `form:"path"`
}

func (s *Service) handlePostOnboarding(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	err := r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		return
	}

	var onboarding = new(onboardingDirector)
	err = decoder.Decode(onboarding, r.Form)
	if err != nil {
		s.logger.WithError(err).Error("failed to decode form")
		s.internalServerError(w)
		return
	}

	switch onboarding.Path {
	case "need":
		s.handleCreateNeed(ctx, w, r)
		return
	}

	data := struct {
		Title string
	}{
		Title: "Onboarding",
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleCreateNeed(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("ctx doesn't contain user")
		s.internalServerError(w)
		return
	}

	need := &types.Need{
		UserID:      userID,
		Status:      types.NeedStatusDraft,
		CurrentStep: types.NeedStepWelcome,
	}

	err = s.needsRepo.CreateNeed(ctx, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to create need in datastore")
		s.internalServerError(w)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/welcome", need.ID), http.StatusSeeOther)
}

type needWelcomeTemplateData struct {
	Title string
	Need  *types.Need
}

func (s *Service) handleGetOnboardingNeedWelcome(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	data := &needWelcomeTemplateData{
		Title: "Need Onboarding",
		Need:  need,
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.welcome", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedWelcome(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepWelcome)
	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/location", need.ID), http.StatusSeeOther)

}

func (s *Service) handleGetOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	data := struct {
		Title string
		*types.Need
	}{
		Title: "Need Location",
		Need:  need,
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.location", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need location page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	err = r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		return
	}

	var location = new(types.NeedLocation)
	err = decoder.Decode(location, r.Form)
	if err != nil {
		s.logger.WithError(err).Error("failed to decode form onto location form")
		s.internalServerError(w)
		return
	}

	need.CurrentStep = types.NeedStepLocation
	need.NeedLocation = location

	err = s.needsRepo.UpdateNeed(ctx, needID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with location data")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepLocation)

	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/categories", need.ID), http.StatusSeeOther)
}

type needCategoriesTemplateData struct {
	Title      string
	Need       *types.Need
	Categories []*types.NeedCategory
}

func (s *Service) handleGetOnboardingNeedCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to load categories from database")
		s.internalServerError(w)
		return
	}

	if len(categories) == 0 {
		s.logger.Warn("no categories found in database - run 'just seed' to populate categories")
	}

	data := &needCategoriesTemplateData{
		Title:      "Select Categories",
		Need:       need,
		Categories: categories,
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.categories", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need categories page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedCategories(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}
	err = r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		return
	}

	if len(r.Form["primary"]) != 1 {
		s.logger.WithError(err).Error("0 or multiple values submitted for primary category")
		s.internalServerError(w)
		return
	}

	ids := make([]string, 0, len(r.Form.Get("secondary"))+1)

	ids = append(ids, r.Form.Get("primary"))
	ids = append(ids, r.Form["secondary"]...)

	categories, err := s.categoryRepo.CategoriesByIDs(ctx, ids)
	if err != nil {
		s.logger.WithError(err).Error("failed to load categories from database")
		s.internalServerError(w)
		return
	}

	// Build a map of category ID to category for easy lookup
	categoryMap := make(map[string]*types.NeedCategory)
	for _, c := range categories {
		categoryMap[c.ID] = c
	}

	needCategories := make([]*types.NeedCategory, 0, len(ids))
	for _, id := range ids {
		cat, ok := categoryMap[id]
		if !ok {
			s.logger.WithField("category_id", id).Error("submitted category ID not found in database")
			s.internalServerError(w)
			return
		}
		needCategories = append(needCategories, cat)
	}

	primaryCategory := categoryMap[r.Form.Get("primary")]
	secondaryCategories := make([]*types.NeedCategory, 0)
	for _, id := range r.Form["secondary"] {
		secondaryCategories = append(secondaryCategories, categoryMap[id])
	}

	err = s.needCategoryAssignmentsRepo.DeleteAllAssignmentsByNeedID(ctx, need.ID)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete existing need category assignments")
		s.internalServerError(w)
		return
	}

	assignments := make([]*types.NeedCategoryAssignment, 0, len(needCategories))
	assignments = append(assignments, &types.NeedCategoryAssignment{
		NeedID:     need.ID,
		CategoryID: primaryCategory.ID,
		IsPrimary:  true,
	})
	for _, cat := range secondaryCategories {
		assignments = append(assignments, &types.NeedCategoryAssignment{
			NeedID:     need.ID,
			CategoryID: cat.ID,
			IsPrimary:  false,
		})
	}

	err = s.needCategoryAssignmentsRepo.CreateAssignments(ctx, assignments)
	if err != nil {
		s.logger.WithError(err).Error("failed to create need category assignments")
		s.internalServerError(w)
		return
	}

	need.CurrentStep = types.NeedStepCategories
	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with selected categories")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepCategories)

	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/story", need.ID), http.StatusSeeOther)
}

func (s *Service) handleGetOnboardingNeedStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	// Get the need
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	// Get all assignments and find primary
	var primaryCategory *types.NeedCategory
	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch category assignments")
		s.internalServerError(w)
		return
	}

	for _, assignment := range assignments {
		if assignment.IsPrimary {
			primaryCategory, err = s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
			if err != nil {
				s.logger.WithError(err).Error("failed to fetch primary category")
				s.internalServerError(w)
				return
			}
			break
		}
	}

	// Get story (may be nil if not yet created)
	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch story")
		s.internalServerError(w)
		return
	}

	data := map[string]interface{}{
		"Title":             "Share Your Story",
		"ID":                needID,
		"AmountNeededCents": need.AmountNeededCents,
		"PrimaryCategory":   primaryCategory,
		"Story":             story,
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.story", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need story page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	// Get existing documents
	documents, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch documents")
		s.internalServerError(w)
		return
	}

	data := map[string]interface{}{
		"Title":               "Upload Documents",
		"ID":                  needID,
		"Documents":           documents,
		"HasDocuments":        len(documents) > 0,
		"Notice":              r.URL.Query().Get("notice"),
		"Error":               r.URL.Query().Get("error"),
		"DocumentTypeOptions": documentTypeOptions(),
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.documents", data)
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
		Bucket:       aws.String(s.config.S3BucketName),
		Key:          aws.String(storageKey),
		Body:         file,
		ContentType:  aws.String(contentType),
		StorageClass: s3t.StorageClassIntelligentTiering,
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
	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/documents?%s", needID, q.Encode()), http.StatusSeeOther)
}

func (s *Service) redirectDocsWithNotice(w http.ResponseWriter, r *http.Request, needID, msg string) {
	q := url.Values{}
	q.Set("notice", msg)
	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/documents?%s", needID, q.Encode()), http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedDocuments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

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

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need")
		s.internalServerError(w)
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
	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/review", needID), http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedDocumentMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	err := r.ParseForm()
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

type needReviewTemplateData struct {
	Title               string
	ID                  string
	Need                *types.Need
	Story               *types.NeedStory
	PrimaryCategory     *types.NeedCategory
	SecondaryCategories []*types.NeedCategory
	Documents           []reviewDocument
	Notice              string
	Error               string
}

type reviewDocument struct {
	ID         string
	FileName   string
	TypeLabel  string
	SizeBytes  int64
	UploadedAt time.Time
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

func (s *Service) handleGetOnboardingNeedReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch story")
		s.internalServerError(w)
		return
	}

	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch category assignments")
		s.internalServerError(w)
		return
	}

	ids := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.CategoryID)
	}

	categoryByID := make(map[string]*types.NeedCategory)
	if len(ids) > 0 {
		categories, err := s.categoryRepo.CategoriesByIDs(ctx, ids)
		if err != nil {
			s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch categories")
			s.internalServerError(w)
			return
		}

		for _, category := range categories {
			categoryByID[category.ID] = category
		}
	}

	var primaryCategory *types.NeedCategory
	secondaryCategories := make([]*types.NeedCategory, 0)
	for _, assignment := range assignments {
		category, ok := categoryByID[assignment.CategoryID]
		if !ok {
			continue
		}

		if assignment.IsPrimary {
			primaryCategory = category
			continue
		}

		secondaryCategories = append(secondaryCategories, category)
	}

	docs, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch documents")
		s.internalServerError(w)
		return
	}

	reviewDocs := make([]reviewDocument, 0, len(docs))
	for _, doc := range docs {
		reviewDocs = append(reviewDocs, reviewDocument{
			ID:         doc.ID,
			FileName:   doc.FileName,
			TypeLabel:  documentTypeLabel(doc.DocumentType),
			SizeBytes:  doc.FileSizeBytes,
			UploadedAt: doc.UploadedAt,
		})
	}

	data := &needReviewTemplateData{
		Title:               "Review Need",
		ID:                  needID,
		Need:                need,
		Story:               story,
		PrimaryCategory:     primaryCategory,
		SecondaryCategories: secondaryCategories,
		Documents:           reviewDocs,
		Notice:              r.URL.Query().Get("notice"),
		Error:               r.URL.Query().Get("error"),
	}

	err = s.templates.ExecuteTemplate(w, "page.onboarding.need.review", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need review page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingNeedConfirmation(w http.ResponseWriter, r *http.Request) {
	needID := r.PathValue("needID")

	data := struct {
		Title string
		ID    string
	}{
		Title: "Need Submitted",
		ID:    needID,
	}

	err := s.templates.ExecuteTemplate(w, "page.onboarding.need.confirmation", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need confirmation page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	// Parse form
	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	// Get the need
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need")
		s.internalServerError(w)
		return
	}

	// Decode story data
	story := &types.NeedStory{
		NeedID: needID,
	}
	if err := decoder.Decode(story, r.Form); err != nil {
		s.logger.WithError(err).Error("failed to decode story form")
		s.internalServerError(w)
		return
	}

	// Parse and convert amount from whole dollars to cents
	amountStr := r.FormValue("amount")
	if amountStr != "" {
		var amountDollars int
		if _, err := fmt.Sscanf(amountStr, "%d", &amountDollars); err == nil {
			need.AmountNeededCents = amountDollars * 100
		}
	}

	// Update need with amount
	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with amount")
		s.internalServerError(w)
		return
	}

	// Upsert story
	err = s.storyRepo.UpsertStory(ctx, story)
	if err != nil {
		s.logger.WithError(err).Error("failed to upsert story")
		s.internalServerError(w)
		return
	}

	// Update current step
	need.CurrentStep = types.NeedStepStory
	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need step")
		s.internalServerError(w)
		return
	}

	// Record progress
	s.recordNeedProgress(ctx, need.ID, types.NeedStepStory)

	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/documents", need.ID), http.StatusSeeOther)
}

func (s *Service) handlePostOnboardingNeedReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := r.PathValue("needID")

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to parse review form")
		s.internalServerError(w)
		return
	}

	if r.FormValue("agreeTerms") != "on" || r.FormValue("agreeVerification") != "on" {
		q := url.Values{}
		q.Set("error", "Please agree to the terms and verification statements before submitting.")
		http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/review?%s", needID, q.Encode()), http.StatusSeeOther)
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to fetch need for review submit")
		s.internalServerError(w)
		return
	}

	now := time.Now()
	need.CurrentStep = types.NeedStepReview
	need.Status = types.NeedStatusSubmitted
	need.SubmittedAt = &now

	err = s.needsRepo.UpdateNeed(ctx, need.ID, need)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to update need status on submit")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepReview)
	http.Redirect(w, r, fmt.Sprintf("/onboarding/need/%s/confirmation", needID), http.StatusSeeOther)
}

// Donor onboarding handlers
func (s *Service) handleGetOnboardingDonorWelcome(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	data := struct {
		Title string
	}{
		Title: "Donor Onboarding",
	}

	err := s.templates.ExecuteTemplate(w, "page.onboarding.donor.welcome", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render donor welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleGetOnboardingDonorPreferences(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	data := struct {
		Title      string
		Categories []any
	}{
		Title: "Donor Preferences",
		Categories: func() []any {
			cats := sampleCategories()
			result := make([]any, len(cats))
			for i, c := range cats {
				result[i] = c
			}
			return result
		}(),
	}

	err := s.templates.ExecuteTemplate(w, "page.onboarding.donor.preferences", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render donor preferences page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingDonorPreferences(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/browse?onboarding=true", http.StatusSeeOther)
}

// Sponsor onboarding handlers (placeholders)
// func (s *Service) handleGetOnboardingSponsorIndividualWelcome(w http.ResponseWriter, r *http.Request) {
// 	var _ = r.Context()

// 	err := s.templates.ExecuteTemplate(w, "page.onboarding.sponsor.individual.welcome", nil)
// 	if err != nil {
// 		s.logger.WithError(err).Error("failed to render sponsor individual welcome page")
// 		s.internalServerError(w)
// 		return
// 	}
// }

// func (s *Service) handleGetOnboardingSponsorOrganizationWelcome(w http.ResponseWriter, r *http.Request) {
// 	var _ = r.Context()

// 	err := s.templates.ExecuteTemplate(w, "page.onboarding.sponsor.organization.welcome", nil)
// 	if err != nil {
// 		s.logger.WithError(err).Error("failed to render sponsor organization welcome page")
// 		s.internalServerError(w)
// 		return
// 	}
// }

func (s *Service) recordNeedProgress(ctx context.Context, needID string, step types.NeedStep) {
	err := s.progressRepo.RecordStepCompletion(ctx, needID, step)
	if err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("step", step).
			Warn("failed to record progress event")
	}
}
