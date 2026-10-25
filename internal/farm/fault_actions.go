package farm

// Classification records are catalogue data: code, scope, and action are kept
// together instead of embedding code comparisons throughout the classifier.
type hmsActionRule struct {
	component uint16
	exactPart bool // Operation-level codes must not match slot-address variants.
	details   []uint32
	guidance  string
}

var (
	printActions = map[string]faultAction{
		"03008013": {userPause: true},
		"03004006": {guidance: checkExtrusion},
		"03008016": {guidance: checkExtrusion},
		"0300801C": {guidance: checkExtrusion},
		"0300801E": {guidance: checkExtrusion},
	}
	printFilamentGuidance = map[uint32]string{
		0x8011: replaceFilament,
		0x8007: checkExtrusion, 0x8013: checkExtrusion,
		0x8003: checkFilamentPath, 0x8004: checkFilamentPath,
		0x8005: checkFilamentPath, 0x8006: checkFilamentPath, 0x8010: checkFilamentPath,
	}
	hmsExactGuidance = map[string]string{
		"07FF_2000_0002_0008": checkFilamentPath,
		"07FF_2000_0002_0009": checkFilamentPath,
		"0300_0900_0002_0001": checkExtrusion,
		"0300_0900_0002_0002": checkExtrusion,
		"0300_0900_0002_0003": checkExtrusion,
		"0300_1A00_0002_0002": checkExtrusion,
		"0300_0100_0001_0007": reviewPrinter,
		"0500_0600_0002_0032": reviewPrinter,
		"0500_0300_0001_0023": reviewPrinter,
		"0500_0300_0001_0025": reviewPrinter,
	}
	hmsActionRules = []hmsActionRule{
		{component: 0x2000, details: []uint32{0x00020001}, guidance: replaceFilament},
		{component: 0x7000, exactPart: true, details: []uint32{0x00020007}, guidance: replaceFilament},
		{component: 0x2000, details: []uint32{0x00020009}, guidance: checkExtrusion},
		{component: 0x7000, exactPart: true, details: []uint32{0x00020003, 0x00020006}, guidance: checkExtrusion},
		{component: 0x1000, details: []uint32{0x00020002}, guidance: checkFilamentPath},
		{component: 0x6000, details: []uint32{0x00020001}, guidance: checkFilamentPath},
		{component: 0x7000, details: []uint32{0x00020001, 0x00020002, 0x00020004, 0x00020005}, guidance: checkFilamentPath},
		{component: 0x2000, details: []uint32{0x00020002, 0x00020003, 0x00020004, 0x00020005, 0x00020010, 0x00020011, 0x00020012, 0x00020024}, guidance: checkFilamentPath},
	}
)
