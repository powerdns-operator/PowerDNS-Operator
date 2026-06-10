package v1alpha2

const (
	FAILED_STATUS        = "Failed"
	PENDING_STATUS       = "Pending"
	SUCCEEDED_STATUS     = "Succeeded"
	UNPROCESSABLE_STATUS = "Unprocessable"
)

const (
	MISSING_ZONE_REASON            = "ZoneMissing"
	MISSING_ZONE_MESSAGE           = "Missing Zone:"
	ZONE_NOT_AVAILABLE_REASON      = "ZoneNotAvailable"
	ZONE_NOT_AVAILABLE_MESSAGE     = "Zone not available:"
	DUPLICATED_REASON              = "Duplicated"
	RRSET_DUPLICATED_MESSAGE       = "At least another ClusterRRset/RRset exists with the same name"
	SYNCHRONIZATION_FAILED_REASON  = "SynchronizationFailed"
	SYNCHRONIZATION_FAILED_MESSAGE = "Synchronization failed:"
	SUCCEEDED_REASON               = "Succeeded"
	SUCCEEDED_MESSAGE              = "Succeeded"
	UNPROCESSABLE_REASON           = "Unprocessable"
	ZONE_DUPLICATED_MESSAGE        = "At least another ClusterZone/Zone exists with the same name"
)
