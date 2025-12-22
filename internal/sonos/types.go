package sonos

import "encoding/xml"

// Device represents a Sonos device.
type Device struct {
	UUID        string
	Name        string
	IPAddress   string
	LocationURL string
	Model       string
	IsReachable bool
	GroupSize   int // Number of visible players in this device's group (1 = standalone, >1 = grouped)
}

// DeviceDescription represents the UPnP device description XML response.
type DeviceDescription struct {
	XMLName xml.Name `xml:"root"`
	Device  struct {
		DeviceType       string `xml:"deviceType"`
		FriendlyName     string `xml:"friendlyName"`
		Manufacturer     string `xml:"manufacturer"`
		ManufacturerURL  string `xml:"manufacturerURL"`
		ModelDescription string `xml:"modelDescription"`
		ModelName        string `xml:"modelName"`
		ModelNumber      string `xml:"modelNumber"`
		ModelURL         string `xml:"modelURL"`
		SerialNum        string `xml:"serialNum"`
		UDN              string `xml:"UDN"`
		RoomName         string `xml:"roomName"`    // Sonos room name (user-configured)
		DisplayName      string `xml:"displayName"` // Sonos display name
	} `xml:"device"`
}

// AVTransportURI contains information for setting the transport URI.
type AVTransportURI struct {
	CurrentURI         string
	CurrentURIMetaData string
}

// PositionInfo contains playback position information.
type PositionInfo struct {
	Track         int
	TrackDuration string // Format: H:MM:SS
	TrackMetaData string
	TrackURI      string
	RelTime       string // Current position: H:MM:SS
	AbsTime       string
	RelCount      int
	AbsCount      int
}

// TransportState represents the playback state.
type TransportState string

const (
	TransportStatePlaying         TransportState = "PLAYING"
	TransportStatePausedPlayback  TransportState = "PAUSED_PLAYBACK"
	TransportStateStopped         TransportState = "STOPPED"
	TransportStateTransitioning   TransportState = "TRANSITIONING"
	TransportStateNoMediaPresent  TransportState = "NO_MEDIA_PRESENT"
)

// TransportInfo contains transport state information.
type TransportInfo struct {
	CurrentTransportState    TransportState
	CurrentTransportStatus   string
	CurrentSpeed             string
}

// SOAPEnvelope is the SOAP envelope structure.
type SOAPEnvelope struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Body    SOAPBody `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
}

// SOAPBody is the SOAP body structure.
type SOAPBody struct {
	Content []byte `xml:",innerxml"`
}

// AVTransportService endpoint path.
const AVTransportServicePath = "/MediaRenderer/AVTransport/Control"

// AVTransport namespace.
const AVTransportNamespace = "urn:schemas-upnp-org:service:AVTransport:1"

// RenderingControl service for volume.
const RenderingControlServicePath = "/MediaRenderer/RenderingControl/Control"
const RenderingControlNamespace = "urn:schemas-upnp-org:service:RenderingControl:1"

// SSDP constants.
const (
	SSDPMulticastAddr = "239.255.255.250:1900"
	SSDPSearchTarget  = "urn:schemas-upnp-org:device:ZonePlayer:1"
)

// ZoneGroupTopology service constants.
const (
	ZoneGroupTopologyServicePath = "/ZoneGroupTopology/Control"
	ZoneGroupTopologyNamespace   = "urn:schemas-upnp-org:service:ZoneGroupTopology:1"
)

// ZoneGroupState represents the parsed zone group topology.
type ZoneGroupState struct {
	ZoneGroups []ZoneGroup
}

// ZoneGroup represents a group of Sonos devices.
type ZoneGroup struct {
	Coordinator string            // UUID of the coordinator
	Members     []ZoneGroupMember // Members in this group
}

// ZoneGroupMember represents a single device in a zone group.
type ZoneGroupMember struct {
	UUID      string // RINCON_XXX format
	ZoneName  string // Room name
	Invisible bool   // True if this is a stereo pair slave
	Location  string // e.g., "http://192.168.1.40:1400/xml/device_description.xml"
	IPAddress string // Extracted from Location
}

// ZoneGroupStateWrapperXML is the outer wrapper after unescaping.
// The structure is: <ZoneGroupState><ZoneGroups>...</ZoneGroups></ZoneGroupState>
type ZoneGroupStateWrapperXML struct {
	XMLName    xml.Name       `xml:"ZoneGroupState"`
	ZoneGroups []ZoneGroupXML `xml:"ZoneGroups>ZoneGroup"`
}

// ZoneGroupsXML is the inner XML structure (for direct parsing if needed).
type ZoneGroupsXML struct {
	XMLName    xml.Name       `xml:"ZoneGroups"`
	ZoneGroups []ZoneGroupXML `xml:"ZoneGroup"`
}

// ZoneGroupXML represents a zone group in the XML.
type ZoneGroupXML struct {
	Coordinator string               `xml:"Coordinator,attr"`
	ID          string               `xml:"ID,attr"`
	Members     []ZoneGroupMemberXML `xml:"ZoneGroupMember"`
}

// ZoneGroupMemberXML represents a zone group member in the XML.
type ZoneGroupMemberXML struct {
	UUID      string `xml:"UUID,attr"`
	ZoneName  string `xml:"ZoneName,attr"`
	Invisible string `xml:"Invisible,attr"` // "0" or "1"
	Location  string `xml:"Location,attr"`  // e.g., "http://192.168.1.40:1400/xml/device_description.xml"
}

// CoordinatorInfo contains information about a group's coordinator.
type CoordinatorInfo struct {
	CoordinatorUUID string // UUID of the coordinator
	CoordinatorIP   string // IP address of the coordinator
	GroupSize       int    // Number of visible members in the group
	IsCoordinator   bool   // True if the queried device is the coordinator
}
