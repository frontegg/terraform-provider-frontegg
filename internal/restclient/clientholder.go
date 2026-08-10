package restclient

type ClientHolder struct {
	ApiClient    Client
	PortalClient Client
	VendorID     string
}
