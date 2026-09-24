package code_sample_setting

// SetModelSamplesForTest replaces the per-model overrides. Tests in other
// packages (model, controller) need to seed this state explicitly because the
// live value is normally populated from the options table at startup.
func SetModelSamplesForTest(samples map[string]map[string]map[string]string) {
	settingLock.Lock()
	defer settingLock.Unlock()
	if samples == nil {
		codeSampleSetting.Models = map[string]map[string]map[string]string{}
		return
	}
	codeSampleSetting.Models = samples
}

// SetTemplatesForTest replaces the site-wide templates.
func SetTemplatesForTest(templates map[string]map[string]string) {
	settingLock.Lock()
	defer settingLock.Unlock()
	if templates == nil {
		codeSampleSetting.Templates = map[string]map[string]string{}
		return
	}
	codeSampleSetting.Templates = templates
}
