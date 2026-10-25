package farm

// Layout of printer/HMS codes; slot bits are removed only for component rules.
// Catalogue entries use literal codes as declarative data alongside their meaning.
const (
	printCodeHexDigits   = 8
	faultWordHexDigits   = 4
	HMSCodeParts         = 4
	HMSPartHexDigits     = 4
	faultWordBits        = 16
	faultWordMask        = 0xffff
	faultPrefixShift     = 24
	hmsPrefixShift       = 56
	hmsPartShift         = 32
	hmsUnitShift         = 16
	hmsSlotShift         = 8
	amsFaultPrefix       = 0x07
	amsHTFaultPrefix     = 0x18
	catalogueUnitCount   = 8
	externalPortFirst    = 0xfe
	externalPortLast     = 0xff
	hmsSlotComponent     = 0x2000
	hmsLastSlotComponent = 0x2300
	hmsComponentMask     = 0xfcff
	hmsFatal             = 1
	hmsSerious           = 2
	hmsCommon            = 3
	hmsInfo              = 4
	hmsAutoPurge         = 0x00030001
	hmsBackupSwitch      = 0x00030002
)
