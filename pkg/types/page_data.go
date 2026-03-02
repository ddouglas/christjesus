package types

import "time"

type NavbarData struct {
	IsAuthenticated bool
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
	FeaturedNeed *Need
	Needs        []*Need
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
}

type NeedDetailPageData struct {
	BasePageData
	Need *Need
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
	Need       *Need
	Categories []*NeedCategory
}

type NeedStoryPageData struct {
	BasePageData
	ID                string
	AmountNeededCents int
	PrimaryCategory   *NeedCategory
	Story             *NeedStory
}

type NeedDocumentsPageData struct {
	BasePageData
	ID                  string
	Documents           []NeedDocument
	HasDocuments        bool
	Notice              string
	Error               string
	DocumentTypeOptions []any
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
	UserID         string
	UserEmail      string
	WelcomeName    string
	UserType       string
	Notice         string
	Error          string
	SidebarItems   []ProfileNavItem
	Needs          []*Need
	NeedSummaries  []ProfileNeedSummary
	DonatedNeeds   []*Need
	HasNeeds       bool
	HasDonatedNeed bool
}

type ProfileNeedSummary struct {
	NeedID              string
	PrimaryCategoryName string
	RequestedAmount     string
	CurrentStep         string
	Status              NeedStatus
	CanDelete           bool
}
