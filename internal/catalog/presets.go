package catalog

const (
	PresetMinimal     = "minimal"
	PresetRecommended = "recommended"
)

type Preset struct {
	ID          string
	Name        string
	Description string
	Choices     map[string]string
}

var presets = []Preset{
	{ID: PresetMinimal, Name: "Minimal (stdlib)", Description: "Standard library only; no extra dependencies beyond the chosen database driver", Choices: map[string]string{}},
	{ID: PresetRecommended, Name: "Recommended", Description: "The recommended option on every step", Choices: map[string]string{}},
}

func Presets() []Preset {
	return append([]Preset(nil), presets...)
}

func PresetByID(id string) (Preset, bool) {
	for _, preset := range presets {
		if preset.ID == id {
			return preset, true
		}
	}
	return Preset{}, false
}
