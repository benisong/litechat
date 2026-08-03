package store

import "testing"

func TestStoryModelSettingsPersist(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()
	config := NewConfigStore(db)
	if err := config.Set("story_compiler_model", "compiler-x"); err != nil {
		t.Fatal(err)
	}
	if err := config.Set("story_scheduler_model", "scheduler-y"); err != nil {
		t.Fatal(err)
	}
	settings, err := config.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.StoryCompilerModel != "compiler-x" || settings.StorySchedulerModel != "scheduler-y" {
		t.Fatalf("story model settings were not persisted: compiler=%q scheduler=%q", settings.StoryCompilerModel, settings.StorySchedulerModel)
	}
}
