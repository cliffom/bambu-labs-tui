# Protocol and message references

This application implements a community-documented cloud monitoring protocol. It is not affiliated with Bambu Lab.

- Account API and MQTT topics: [OpenBambuAPI](https://github.com/Doridian/OpenBambuAPI).
- HMS severity encoding: [Bambu Studio DevHMS.h](https://github.com/bambulab/BambuStudio/blob/master/src/slic3r/GUI/DeviceCore/DevHMS.h) and [DevHMS.cpp](https://github.com/bambulab/BambuStudio/blob/master/src/slic3r/GUI/DeviceCore/DevHMS.cpp). The upper word of the code encodes fatal (1), serious (2), common (3), or informational (4). The TUI groups fatal/serious as errors, common as warnings, and informational as info. Unrecognized levels remain visible as warnings.
- Message meanings: Bambu Studio's English [093](https://github.com/bambulab/BambuStudio/blob/master/resources/hms/hms_en_093.json), [094](https://github.com/bambulab/BambuStudio/blob/master/resources/hms/hms_en_094.json), [20P](https://github.com/bambulab/BambuStudio/blob/master/resources/hms/hms_en_20P.json), and [31B](https://github.com/bambulab/BambuStudio/blob/master/resources/hms/hms_en_31B.json) catalogues, inspected 2026-10-23. The bundled short descriptions are independently phrased summaries, not a copy of those catalogues. Hardware-specific wording is generalized when necessary.
- `0500_0200_0003_000A` has an empty description in the 20P catalogue. It is suppressed when no explicit message is supplied in telemetry. All other unmapped codes retain their code and severity with an “Unrecognized HMS notice” label.
- Dual-nozzle temperatures, filament paths, and AMS HT IDs: [Home Assistant Bambu integration](https://github.com/greghesp/ha-bambulab/tree/main/custom_components/bambu_lab/pybambu). Protocol IDs 128–135 are shown as AMS HT 1–8; wire identities are not changed.
- Native credential CLI: [Apple SecurityTool.c](https://github.com/apple-oss-distributions/Security/blob/main/SecurityTool/sharedTool/SecurityTool.c). The interactive parser has a 4096-byte buffer; oversized writes are rejected, and EOF preserves the add command's failure status. No credential appears in process arguments.

Unknown messages are not guessed, and no telemetry fault-clear operation is implemented. Descriptions can differ across printer models and firmware; use the displayed code to check Bambu's troubleshooting information.
