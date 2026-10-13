package mcpclient

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The read surface is defined by a pattern, so pin one representative of every
// pattern and the three Phase 3 names that never existed upstream.
func TestReadOnlyDeniesWritesAndUnneededGroups(t *testing.T) {
	denied := []string{
		"set_activity_name", "add_weigh_in", "create_run_workout", "delete_activity",
		"upload_workout", "remove_gear_from_activity", "log_food", "update_custom_food",
		"download_activity_file", "schedule_workout", "unschedule_workouts", "upsert_and_log",
		"request_reload", "set_fit_download_dir", "search_foods",
		"get_nutrition_summary_between_dates", "get_custom_foods", "get_hydration_data",
		"get_earned_badges", "get_adhoc_challenges", "get_gear", "get_courses",
		"get_workouts", "get_scheduled_workouts", "get_menstrual_data_for_date",
		"get_pregnancy_summary", "get_device_alarms", "get_blood_pressure",
	}
	for _, name := range denied {
		if ReadOnly(name) {
			t.Errorf("%s should be denied", name)
		}
	}
	allowed := []string{
		"get_activities", "get_activities_by_date", "get_activity_splits", "get_sleep_data",
		"get_sleep_summary_range", "get_hrv_trend", "get_hrv_data", "get_body_battery",
		"get_stress_summary", "get_all_day_stress", "get_training_load_trend",
		"get_training_status", "get_training_readiness", "get_vo2max_trend",
		"get_race_predictions", "get_user_profile", "get_daily_steps", "get_body_composition",
		"get_devices", "get_primary_training_device", "get_activity_fit_data", "count_activities",
	}
	for _, name := range allowed {
		if !ReadOnly(name) {
			t.Errorf("%s should be allowed", name)
		}
	}
}

// Applied to the pinned upstream list, the pattern must leave the read surface
// the plan describes: roughly eighty tools (77 at the pinned commit), none of them a write.
func TestReadOnlyOverUpstreamList(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "deploy", "garmin-mcp", "tools.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open upstream tool list: %v", err)
	}
	defer f.Close()

	var total, exposed int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		total++
		if ReadOnly(name) {
			exposed++
			if !strings.HasPrefix(name, "get_") && name != "count_activities" {
				t.Errorf("exposed tool %q is not a read", name)
			}
		}
	}
	if total != 155 {
		t.Errorf("upstream list has %d names, want 155 at the pinned commit", total)
	}
	if exposed < 70 || exposed > 100 {
		t.Errorf("read surface is %d tools, want between 70 and 100", exposed)
	}
	t.Logf("read surface: %d of %d upstream tools", exposed, total)
}

// The spawned server must never be able to sign in to Garmin, whatever the
// parent process has in its environment (ADR 0003).
func TestChildEnvNeverCarriesGarminCredentials(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin", "HOME=/home/leo", "GARMINTOKENS=/tmp/tokens",
		"GARMIN_EMAIL=leo@example.com", "GARMIN_PASSWORD=hunter2",
		"ANTHROPIC_API_KEY=sk-ant-secret", "DATABASE_URL=postgres://x",
		"RUNCOACH_TOKEN_KEY=abc", "DOCKER_HOST=unix:///var/run/docker.sock",
	}
	got := childEnv(parent)
	want := []string{"PATH=/usr/bin", "HOME=/home/leo", "GARMINTOKENS=/tmp/tokens", "DOCKER_HOST=unix:///var/run/docker.sock"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("child env:\n got %v\nwant %v", got, want)
	}
	for _, kv := range got {
		if strings.HasPrefix(kv, "GARMIN_EMAIL=") || strings.HasPrefix(kv, "GARMIN_PASSWORD=") {
			t.Fatalf("credential leaked into the child environment: %s", kv)
		}
	}
}

// Upstream reports most failures as plain text with is_error false.
func TestLooksLikeError(t *testing.T) {
	for _, s := range []string{
		"Error retrieving user profile: Garmin login failed. Run 'garmin-mcp-auth' to authenticate, then restart the server.",
		"  Error fetching sleep data: 429 Client Error",
		"OAuth tokens not found and no interactive terminal available.",
	} {
		if !LooksLikeError(s) {
			t.Errorf("%q should read as an error", s)
		}
	}
	for _, s := range []string{`{"displayName":"leo"}`, "[]", "No activities found for that range (an empty result is not an error)"} {
		if LooksLikeError(s) {
			t.Errorf("%q should not read as an error", s)
		}
	}
}
