package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"christjesus/internal/utils"
	"christjesus/pkg/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type needReviewCoreData struct {
	Need                *types.Need
	Story               *types.NeedStory
	PrimaryCategory     *types.NeedCategory
	SecondaryCategories []*types.NeedCategory
	SelectedAddress     *types.UserAddress
	Documents           []types.ReviewDocument
}

func (s *Service) handleGetProfileNeedEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		http.NotFound(w, r)
		return
	}

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	http.Redirect(w, r, s.route(RouteProfileNeedEditLocation, map[string]string{"needID": needID}), http.StatusSeeOther)
}

func (s *Service) handleGetProfileNeedEditLocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	addresses, err := s.userAddressRepo.AddressesByUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch user addresses")
		s.internalServerError(w)
		return
	}

	selectedAddressID := ""
	if need.UserAddressID != nil && *need.UserAddressID != "" {
		selectedAddressID = *need.UserAddressID
	} else if len(addresses) > 0 {
		selectedAddressID = addresses[0].ID
	} else {
		selectedAddressID = "new"
	}

	showSetSelectedPrimary := false
	for _, address := range addresses {
		if address.ID == selectedAddressID {
			showSetSelectedPrimary = !address.IsPrimary
			break
		}
	}

	data := &types.NeedLocationPageData{
		BasePageData:      types.BasePageData{Title: "Edit Need Location"},
		ID:                needID,
		Addresses:         addresses,
		HasAddresses:      len(addresses) > 0,
		SelectedAddressID: selectedAddressID,
		ShowSetPrimary:    showSetSelectedPrimary,
		NewAddress:        &types.UserAddressForm{},
		FormAction:        s.route(RouteProfileNeedEditLocation, map[string]string{"needID": needID}),
		BackHref:          s.route(RouteProfileNeedReview, map[string]string{"needID": needID}),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.location", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need location edit page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditLocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	addresses, err := s.userAddressRepo.AddressesByUserID(ctx, userID)
	if err != nil {
		s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch user addresses")
		s.internalServerError(w)
		return
	}

	selection := strings.TrimSpace(r.FormValue("address_selection"))
	if selection == "" && len(addresses) > 0 {
		selection = addresses[0].ID
	}
	if selection == "" && len(addresses) == 0 {
		selection = "new"
	}

	var selectedAddress *types.UserAddress
	usesNonPrimaryAddress := false

	if selection != "new" {
		addressID := selection
		if addressID == "" {
			s.logger.Error("missing selected address id")
			s.internalServerError(w)
			return
		}

		selectedAddress, err = s.userAddressRepo.ByIDAndUserID(ctx, addressID, userID)
		if err != nil {
			s.logger.WithError(err).WithField("address_id", addressID).Error("failed to fetch selected user address")
			s.internalServerError(w)
			return
		}

		if selectedAddress == nil {
			s.logger.WithField("address_id", addressID).Error("selected user address not found")
			s.internalServerError(w)
			return
		}

		setSelectedAsPrimary := r.FormValue("set_selected_as_primary") == "on"
		if setSelectedAsPrimary && !selectedAddress.IsPrimary {
			err = s.userAddressRepo.SetPrimaryByID(ctx, userID, selectedAddress.ID)
			if err != nil {
				s.logger.WithError(err).WithField("address_id", selectedAddress.ID).Error("failed to promote selected address to primary")
				s.internalServerError(w)
				return
			}
			selectedAddress.IsPrimary = true
		}

		usesNonPrimaryAddress = !selectedAddress.IsPrimary
	} else {
		location := new(types.UserAddressForm)
		err = decoder.Decode(location, r.Form)
		if err != nil {
			s.logger.WithError(err).Error("failed to decode form onto location form")
			s.internalServerError(w)
			return
		}

		if location.Address == nil || strings.TrimSpace(*location.Address) == "" ||
			location.City == nil || strings.TrimSpace(*location.City) == "" ||
			location.State == nil || strings.TrimSpace(*location.State) == "" ||
			location.ZipCode == nil || strings.TrimSpace(*location.ZipCode) == "" {
			s.logger.Error("new address submission missing required fields")
			s.internalServerError(w)
			return
		}

		setNewAsPrimary := len(addresses) == 0 || r.FormValue("set_new_as_primary") == "on"

		selectedAddress = &types.UserAddress{
			ID:                   utils.NanoID(),
			UserID:               userID,
			Address:              location.Address,
			AddressExt:           location.AddressExt,
			City:                 location.City,
			State:                location.State,
			ZipCode:              location.ZipCode,
			PrivacyDisplay:       location.PrivacyDisplay,
			ContactMethods:       location.ContactMethods,
			PreferredContactTime: location.PreferredContactTime,
			IsPrimary:            setNewAsPrimary,
		}

		err = s.userAddressRepo.Create(ctx, selectedAddress)
		if err != nil {
			s.logger.WithError(err).WithField("user_id", userID).Error("failed to create user address")
			s.internalServerError(w)
			return
		}

		usesNonPrimaryAddress = !setNewAsPrimary
	}

	need.CurrentStep = types.NeedStepLocation
	need.UserAddressID = &selectedAddress.ID
	need.UsesNonPrimaryAddress = usesNonPrimaryAddress

	err = s.needsRepo.UpdateNeed(ctx, needID, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to update need with location data")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepLocation)
	http.Redirect(w, r, s.route(RouteProfileNeedEditCategories, map[string]string{"needID": need.ID}), http.StatusSeeOther)
}

func (s *Service) handleGetProfileNeedEditCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
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

	selectedPrimaryCategoryID, selectedSecondaryCategoryIDs, err := s.selectedNeedCategories(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load selected categories for profile edit")
		s.internalServerError(w)
		return
	}

	data := &types.NeedCategoriesPageData{
		BasePageData:                 types.BasePageData{Title: "Edit Need Categories"},
		Need:                         need,
		Categories:                   categories,
		SelectedPrimaryCategoryID:    selectedPrimaryCategoryID,
		SelectedSecondaryCategoryIDs: selectedSecondaryCategoryIDs,
		FormAction:                   s.route(RouteProfileNeedEditCategories, map[string]string{"needID": needID}),
		BackHref:                     s.route(RouteProfileNeedEditLocation, map[string]string{"needID": needID}),
		Error:                        strings.TrimSpace(r.URL.Query().Get("error")),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.categories", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need categories edit page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	err = r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	if len(r.Form["primary"]) != 1 {
		s.redirectProfileNeedEditCategoriesWithError(w, r, needID, "Please select exactly one primary category.")
		return
	}

	primaryID := strings.TrimSpace(r.Form.Get("primary"))
	if primaryID == "" {
		s.redirectProfileNeedEditCategoriesWithError(w, r, needID, "Please select exactly one primary category.")
		return
	}

	secondaryIDs := make([]string, 0, len(r.Form["secondary"]))
	seenSecondaryIDs := make(map[string]bool, len(r.Form["secondary"]))
	for _, rawID := range r.Form["secondary"] {
		categoryID := strings.TrimSpace(rawID)
		if categoryID == "" || categoryID == primaryID || seenSecondaryIDs[categoryID] {
			continue
		}
		seenSecondaryIDs[categoryID] = true
		secondaryIDs = append(secondaryIDs, categoryID)
	}

	ids := make([]string, 0, len(secondaryIDs)+1)
	ids = append(ids, primaryID)
	ids = append(ids, secondaryIDs...)

	categories, err := s.categoryRepo.CategoriesByIDs(ctx, ids)
	if err != nil {
		s.logger.WithError(err).Error("failed to load categories from database")
		s.internalServerError(w)
		return
	}

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

	primaryCategory := categoryMap[primaryID]
	secondaryCategories := make([]*types.NeedCategory, 0, len(secondaryIDs))
	for _, id := range secondaryIDs {
		secondaryCategories = append(secondaryCategories, categoryMap[id])
	}

	err = s.needCategoryAssignmentsRepo.DeleteAllAssignmentsByNeedID(ctx, need.ID)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete existing need category assignments")
		s.internalServerError(w)
		return
	}

	assignments := make([]*types.NeedCategoryAssignment, 0, len(needCategories))
	assignments = append(assignments, &types.NeedCategoryAssignment{NeedID: need.ID, CategoryID: primaryCategory.ID, IsPrimary: true})
	for _, cat := range secondaryCategories {
		assignments = append(assignments, &types.NeedCategoryAssignment{NeedID: need.ID, CategoryID: cat.ID, IsPrimary: false})
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
	http.Redirect(w, r, s.route(RouteProfileNeedEditStory, map[string]string{"needID": need.ID}), http.StatusSeeOther)
}

func (s *Service) redirectProfileNeedEditCategoriesWithError(w http.ResponseWriter, r *http.Request, needID, msg string) {
	q := url.Values{}
	q.Set("error", msg)
	http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedEditCategories, map[string]string{"needID": needID}, q), http.StatusSeeOther)
}

func (s *Service) handleGetProfileNeedEditStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	var primaryCategory *types.NeedCategory
	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch category assignments")
		s.internalServerError(w)
		return
	}

	for _, assignment := range assignments {
		if !assignment.IsPrimary {
			continue
		}

		primaryCategory, err = s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
		if err != nil {
			s.logger.WithError(err).Error("failed to fetch primary category")
			s.internalServerError(w)
			return
		}
		break
	}

	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch story")
		s.internalServerError(w)
		return
	}

	data := &types.NeedStoryPageData{
		BasePageData:      types.BasePageData{Title: "Edit Need Story"},
		ID:                needID,
		AmountNeededCents: need.AmountNeededCents,
		PrimaryCategory:   primaryCategory,
		Story:             story,
		FormAction:        s.route(RouteProfileNeedEditStory, map[string]string{"needID": needID}),
		BackHref:          s.route(RouteProfileNeedEditCategories, map[string]string{"needID": needID}),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.story", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need story edit page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditStory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		s.internalServerError(w)
		return
	}

	story := &types.NeedStory{NeedID: needID}
	if err := decoder.Decode(story, r.Form); err != nil {
		s.logger.WithError(err).Error("failed to decode story form")
		s.internalServerError(w)
		return
	}

	amountStr := r.FormValue("amount")
	if amountStr != "" {
		var amountDollars int
		if _, err := fmt.Sscanf(amountStr, "%d", &amountDollars); err == nil {
			need.AmountNeededCents = amountDollars * 100
		}
	}

	if err := s.needsRepo.UpdateNeed(ctx, need.ID, need); err != nil {
		s.logger.WithError(err).Error("failed to update need with amount")
		s.internalServerError(w)
		return
	}

	if err := s.storyRepo.UpsertStory(ctx, story); err != nil {
		s.logger.WithError(err).Error("failed to upsert story")
		s.internalServerError(w)
		return
	}

	need.CurrentStep = types.NeedStepStory
	if err := s.needsRepo.UpdateNeed(ctx, need.ID, need); err != nil {
		s.logger.WithError(err).Error("failed to update need step")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepStory)
	http.Redirect(w, r, s.route(RouteProfileNeedEditDocs, map[string]string{"needID": need.ID}), http.StatusSeeOther)
}

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

func (s *Service) handleGetProfileNeedEditReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	if _, err := s.profileEditableNeed(ctx, needID); err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	core, err := s.loadNeedReviewCoreData(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load need review core data")
		s.internalServerError(w)
		return
	}

	data := &types.NeedReviewPageData{
		BasePageData:        types.BasePageData{Title: "Edit Need Review"},
		ID:                  needID,
		Need:                core.Need,
		SelectedAddress:     core.SelectedAddress,
		Story:               core.Story,
		PrimaryCategory:     core.PrimaryCategory,
		SecondaryCategories: core.SecondaryCategories,
		Documents:           core.Documents,
		EditLocationHref:    s.route(RouteProfileNeedEditLocation, map[string]string{"needID": needID}),
		EditCategoriesHref:  s.route(RouteProfileNeedEditCategories, map[string]string{"needID": needID}),
		EditStoryHref:       s.route(RouteProfileNeedEditStory, map[string]string{"needID": needID}),
		EditDocumentsHref:   s.route(RouteProfileNeedEditDocs, map[string]string{"needID": needID}),
		SubmitAction:        s.route(RouteProfileNeedEditReview, map[string]string{"needID": needID}),
		BackHref:            s.route(RouteProfileNeedEditDocs, map[string]string{"needID": needID}),
		SubmitLabel:         "Submit Updated Need",
		Notice:              strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:               strings.TrimSpace(r.URL.Query().Get("error")),
	}

	if err := s.renderTemplate(w, r, "page.onboarding.need.review", data); err != nil {
		s.logger.WithError(err).Error("failed to render profile need review page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedEditReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	needID := strings.TrimSpace(r.PathValue("needID"))

	need, err := s.profileEditableNeed(ctx, needID)
	if err != nil {
		s.handleProfileEditableNeedError(w, r, needID, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to parse review form")
		s.internalServerError(w)
		return
	}

	if r.FormValue("agreeTerms") != "on" || r.FormValue("agreeVerification") != "on" {
		q := url.Values{}
		q.Set("error", "Please agree to the terms and verification statements before submitting.")
		http.Redirect(w, r, s.routeWithQuery(RouteProfileNeedEditReview, map[string]string{"needID": needID}, q), http.StatusSeeOther)
		return
	}

	now := time.Now()
	need.CurrentStep = types.NeedStepReview
	need.Status = types.NeedStatusReadyForReview
	need.SubmittedAt = &now

	if err := s.needsRepo.UpdateNeed(ctx, need.ID, need); err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to update need status on profile edit submit")
		s.internalServerError(w)
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepReview)
	s.redirectProfileNeedReviewWithNotice(w, r, needID, "Updated need submitted for review.")
}

func (s *Service) profileEditableNeed(ctx context.Context, needID string) (*types.Need, error) {
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		return nil, err
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if need.UserID != userID {
		return nil, types.ErrNeedNotFound
	}

	if need.Status != types.NeedStatusSubmitted && need.Status != types.NeedStatusReadyForReview && need.Status != types.NeedStatusChangesRequested {
		return nil, fmt.Errorf("need is not editable in its current state")
	}

	return need, nil
}

func (s *Service) loadNeedReviewCoreData(ctx context.Context, needID string) (*needReviewCoreData, error) {
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		return nil, err
	}

	story, err := s.storyRepo.GetStoryByNeedID(ctx, needID)
	if err != nil {
		return nil, err
	}

	assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, needID)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.CategoryID)
	}

	categoryByID := make(map[string]*types.NeedCategory)
	if len(ids) > 0 {
		categories, err := s.categoryRepo.CategoriesByIDs(ctx, ids)
		if err != nil {
			return nil, err
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

	var selectedAddress *types.UserAddress
	if need.UserAddressID != nil && *need.UserAddressID != "" {
		selectedAddress, err = s.userAddressRepo.ByIDAndUserID(ctx, *need.UserAddressID, need.UserID)
		if err != nil {
			return nil, err
		}
	}
	if selectedAddress == nil {
		selectedAddress, err = s.userAddressRepo.PrimaryByUserID(ctx, need.UserID)
		if err != nil {
			return nil, err
		}
	}

	docs, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		return nil, err
	}

	reviewDocs := make([]types.ReviewDocument, 0, len(docs))
	for _, doc := range docs {
		reviewDocs = append(reviewDocs, types.ReviewDocument{ID: doc.ID, FileName: doc.FileName, TypeLabel: documentTypeLabel(doc.DocumentType), SizeBytes: doc.FileSizeBytes, UploadedAt: doc.UploadedAt})
	}

	return &needReviewCoreData{
		Need:                need,
		Story:               story,
		PrimaryCategory:     primaryCategory,
		SecondaryCategories: secondaryCategories,
		SelectedAddress:     selectedAddress,
		Documents:           reviewDocs,
	}, nil
}

func (s *Service) needDocumentDeleteActions(routeName RouteName, needID string, documentIDs []string) map[string]string {
	actions := make(map[string]string, len(documentIDs))
	for _, documentID := range documentIDs {
		actions[documentID] = s.route(routeName, map[string]string{"needID": needID, "documentID": documentID})
	}
	return actions
}

func needDocumentIDs(documents []types.NeedDocument) []string {
	ids := make([]string, 0, len(documents))
	for _, document := range documents {
		ids = append(ids, document.ID)
	}
	return ids
}

func (s *Service) handleProfileEditableNeedError(w http.ResponseWriter, r *http.Request, needID string, err error) {
	if err == types.ErrNeedNotFound {
		http.NotFound(w, r)
		return
	}

	if strings.Contains(strings.ToLower(err.Error()), "current state") {
		s.redirectProfileNeedReviewWithError(w, r, needID, "This need cannot be edited in its current status.")
		return
	}

	s.logger.WithError(err).WithField("need_id", needID).Error("failed to load editable need")
	s.internalServerError(w)
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
