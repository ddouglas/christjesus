package types

import "time"

type NavbarData struct {
	IsAuthenticated bool
	IsAdmin         bool
	UserID          string
	UserEmail       string
	UserName        string
	AvatarURL       string
}

type NavbarDataSetter interface {
	SetNavbarData(data NavbarData)
}

type BasePageData struct {
	Title  string
	Navbar NavbarData
}

func (d *BasePageData) SetNavbarData(data NavbarData) {
	d.Navbar = data
}

type HomePageData struct {
	BasePageData
	Notice       string
	Error        string
	FeaturedNeed *BrowseNeedCard
	Needs        []*BrowseNeedCard
	Categories   []CategoryData
	Stats        StatsData
	Steps        []StepData
}

type BrowsePageData struct {
	BasePageData
	Needs                []*BrowseNeedCard
	Categories           []*NeedCategory
	Cities               []string
	Filters              BrowseFilters
	LoadResultsOnRender  bool
	ShowResultsSkeletons bool
}

type BrowseFilters struct {
	Search          string
	City            string
	CategoryIDs     map[string]bool
	VerificationIDs map[string]bool
	Urgency         string
	FundingMax      int
	ViewMode        string
	SortBy          string
}

type BrowseNeedCard struct {
	ID                string
	OwnerName         string
	City              string
	State             string
	CityState         string
	UrgencyLabel      string
	UrgencyDotClass   string
	PrimaryCategoryID string
	PrimaryCategory   string
	VerificationID    string
	VerificationLabel string
	ShortDescription  *string
	Status            NeedStatus
	AmountNeededCents int
	AmountRaisedCents int
	FundingPercent    int
	CreatedAt         time.Time
}

type CategoriesPageData struct {
	BasePageData
	Categories []*CategoryListItem
	BrowseHref string
}

type CategoryListItem struct {
	ID          string
	Name        string
	Slug        string
	Description *string
	Icon        *string
	NeedCount   int
	Href        string
}

type CategoryNeedsPageData struct {
	BasePageData
	Category   *NeedCategory
	Needs      []*BrowseNeedCard
	BackHref   string
	BrowseHref string
}

type NeedDetailPageData struct {
	BasePageData
	ID                  string
	Need                *Need
	OwnerName           string
	SelectedAddress     *UserAddress
	CityState           string
	UrgencyLabel        string
	UrgencyDotClass     string
	VerificationLabel   string
	FundingPercent      int
	Story               *NeedStory
	PrimaryCategory     *NeedCategory
	SecondaryCategories []*NeedCategory
	Documents           []ReviewDocument
	RelatedNeeds        []*BrowseNeedCard
}

type NeedDonatePageData struct {
	BasePageData
	NeedID            string
	OwnerName         string
	PrimaryCategory   string
	ShortDescription  *string
	AmountNeededCents int
	AmountRaisedCents int
	SelectedPreset    int
	CustomAmount      string
	PrivateMessage    string
	IsAnonymous       bool
	Error             string
	PresetAmounts     []int
}

type NeedDonateConfirmationPageData struct {
	BasePageData
	NeedID             string
	IntentID           string
	OwnerName          string
	AmountCents        int
	IsAnonymous        bool
	PrimaryCategory    string
	PaymentStatus      string
	StatusLabel        string
	StatusTitle        string
	StatusDescription  string
	StatusGuidance     string
	ShowRetryCTA       bool
	ShowReceiptDetails bool
	DonationDate       string
}

type LoginPageData struct {
	BasePageData
	Message string
	Error   string
	Email   string
}

type RegisterPageData struct {
	BasePageData
	GivenName   string
	FamilyName  string
	Email       string
	Error       string
	FieldErrors map[string]string
}

type ConfirmRegisterPageData struct {
	BasePageData
	Email   string
	Error   string
	Message string
}

type OnboardingPageData struct {
	BasePageData
}

type NeedWelcomePageData struct {
	BasePageData
	Need *Need
}

type NeedCategoriesPageData struct {
	BasePageData
	Need                         *Need
	Categories                   []*NeedCategory
	SelectedPrimaryCategoryID    string
	SelectedSecondaryCategoryIDs map[string]bool
	FormAction                   string
	BackHref                     string
	Error                        string
}

type NeedStoryPageData struct {
	BasePageData
	ID                string
	AmountNeededCents int
	PrimaryCategory   *NeedCategory
	Story             *NeedStory
	FormAction        string
	BackHref          string
}

type NeedDocumentsPageData struct {
	BasePageData
	ID                  string
	Documents           []NeedDocument
	HasDocuments        bool
	Notice              string
	Error               string
	DocumentTypeOptions []any
	MetadataAction      string
	UploadAction        string
	ContinueAction      string
	BackHref            string
	DeleteActions       map[string]string
}

type ReviewDocument struct {
	ID         string
	FileName   string
	TypeLabel  string
	SizeBytes  int64
	UploadedAt time.Time
}

type NeedReviewPageData struct {
	BasePageData
	ID                  string
	Need                *Need
	SelectedAddress     *UserAddress
	Story               *NeedStory
	PrimaryCategory     *NeedCategory
	SecondaryCategories []*NeedCategory
	Documents           []ReviewDocument
	EditLocationHref    string
	EditCategoriesHref  string
	EditStoryHref       string
	EditDocumentsHref   string
	SubmitAction        string
	BackHref            string
	SubmitLabel         string
	Notice              string
	Error               string
}

type UserAddressForm struct {
	Address              *string  `form:"address"`
	AddressExt           *string  `form:"address_ext"`
	City                 *string  `form:"city"`
	State                *string  `form:"state"`
	ZipCode              *string  `form:"zip_code"`
	PrivacyDisplay       *string  `form:"privacy_display"`
	ContactMethods       []string `form:"contact_methods"`
	PreferredContactTime *string  `form:"preferred_contact_time"`
}

type NeedLocationPageData struct {
	BasePageData
	ID                string
	Addresses         []*UserAddress
	HasAddresses      bool
	SelectedAddressID string
	ShowSetPrimary    bool
	NewAddress        *UserAddressForm
	FormAction        string
	BackHref          string
}

type NeedSubmittedPageData struct {
	BasePageData
	ID string
}

type DonorWelcomePageData struct {
	BasePageData
}

type DonorPreferencesPageData struct {
	BasePageData
	Categories            []*NeedCategory
	ZipCode               string
	Radius                string
	DonationRange         string
	NotificationFrequency string
	SelectedCategoryIDs   map[string]bool
	Notice                string
	Error                 string
}

type DonorConfirmationPageData struct {
	BasePageData
}

type ProfileNavItem struct {
	Label    string
	Href     string
	Active   bool
	Section  string
	ShowItem bool
}

type ProfilePageData struct {
	BasePageData
	UserID            string
	UserEmail         string
	WelcomeName       string
	UserType          string
	Notice            string
	Error             string
	SidebarItems      []ProfileNavItem
	Needs             []*Need
	NeedSummaries     []ProfileNeedSummary
	DonationSummaries []ProfileDonationSummary
	HasNeeds          bool
	HasDonations      bool
}

type ProfileNeedSummary struct {
	NeedID              string
	PrimaryCategoryName string
	RequestedAmount     string
	CurrentStep         string
	Status              NeedStatus
	CanDelete           bool
	NeedsAttention      bool
	ReviewPortalHref    string
}

type NeedReviewMessageView struct {
	ID           string
	AuthorLabel  string
	Body         string
	CreatedAt    string
	IsFromAdmin  bool
	IsFromViewer bool
}

type NeedReviewDocumentFeedback struct {
	DocumentID string
	FileName   string
	TypeLabel  string
	Status     string
	Reason     string
	Note       string
	ViewHref   string
}

type NeedReviewPortalPageData struct {
	BasePageData
	Need                *Need
	Story               *NeedStory
	PrimaryCategory     *NeedCategory
	SecondaryCategories []*NeedCategory
	RejectionReason     string
	RejectionNote       string
	Documents           []NeedReviewDocumentFeedback
	Messages            []NeedReviewMessageView
	PostMessageAction   string
	SetReadyAction      string
	PullBackAction      string
	BackHref            string
	EditNeedHref        string
	CanEditNeed         bool
	CanSetReady         bool
	CanPullBack         bool
	CanSendMessage      bool
	Notice              string
	Error               string
}

type ProfileDonationSummary struct {
	IntentID    string
	NeedID      string
	NeedLabel   string
	Amount      string
	Status      string
	IsFinalized bool
	IsAnonymous bool
	CreatedAt   string
}

type AdminDashboardPageData struct {
	BasePageData
}

type AdminNeedsPageData struct {
	BasePageData
	Needs      []*AdminNeedQueueItem
	Page       int
	PageSize   int
	TotalNeeds int
	TotalPages int
	PrevHref   string
	NextHref   string
}

type AdminNeedQueueItem struct {
	NeedID      string
	Status      NeedStatus
	CreatedAt   string
	SubmittedAt string
	ReviewHref  string
}

type AdminNeedExplorerPageData struct {
	BasePageData
	Needs             []*AdminNeedExplorerItem
	StatusCards       []*AdminNeedStatusCard
	Page              int
	PageSize          int
	TotalNeeds        int
	TotalPages        int
	PrevHref          string
	NextHref          string
	SelectedStatus    string
	SelectedSort      string
	FilterAction      string
	StatusOptions     []AdminExplorerOption
	SortOptions       []AdminExplorerOption
	BackHref          string
	QueueHref         string
	CurrentStatusText string
}

type AdminNeedStatusCard struct {
	Status  NeedStatus
	Label   string
	Count   int
	Href    string
	IsActive bool
}

type AdminExplorerOption struct {
	Value string
	Label string
}

type AdminNeedExplorerItem struct {
	NeedID            string
	Status            NeedStatus
	AmountRaisedCents int
	AmountNeededCents int
	FundingPercent    int
	ActivityLabel     string
	UpdatedAt         string
	PublishedAt       string
	ReviewHref        string
	DetailHref        string
	CanViewDetail     bool
}

type AdminNeedReviewPageData struct {
	BasePageData
	Need                *Need
	Story               *NeedStory
	PrimaryCategory     *NeedCategory
	SecondaryCategories []*NeedCategory
	SelectedAddress     *UserAddress
	CityState           string
	Documents           []*AdminNeedReviewDocument
	Timeline            []*AdminNeedTimelineItem
	BackHref            string
	ModerateAction      string
	AcceptReviewAction  string
	CanAcceptReview     bool
	CanSubmitModeration bool
	DeleteAction        string
	RestoreAction       string
	IsDeleted           bool
	DeletedAt           string
	DeletedByUserID     string
	DeleteReason        string
	Messages            []NeedReviewMessageView
	MessageAction       string
	Notice              string
	Error               string
}

type AdminNeedReviewDocument struct {
	ID          string
	FileName    string
	TypeLabel   string
	UploadedAt  string
	Status      string
	MimeType    string
	FileSize    string
	PreviewHref string
}

type AdminNeedTimelineItem struct {
	When       string
	Step       string
	Actor      string
	Source     string
	ActionType string
	Reason     string
	Note       string
	DocumentID string
}
