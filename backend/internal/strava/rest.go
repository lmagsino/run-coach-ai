package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const apiBase = "https://www.strava.com/api/v3"

// Activity is a normalized subset of a Strava summary activity.
type Activity struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Type               string  `json:"type"`
	SportType          string  `json:"sport_type"`
	StartDate          string  `json:"start_date"`
	StartDateLocal     string  `json:"start_date_local"`
	DistanceM          float64 `json:"distance"`
	MovingTimeS        int     `json:"moving_time"`
	ElapsedTimeS       int     `json:"elapsed_time"`
	TotalElevationGain float64 `json:"total_elevation_gain"`
	AverageSpeedMS     float64 `json:"average_speed"`
	MaxSpeedMS         float64 `json:"max_speed"`
	AverageHeartrate   float64 `json:"average_heartrate"`
	MaxHeartrate       float64 `json:"max_heartrate"`
}

// ListActivitiesParams filters the activity list. Zero values are omitted.
type ListActivitiesParams struct {
	Before  *time.Time
	After   *time.Time
	Page    int
	PerPage int
}

// ListActivities calls GET /athlete/activities with the given bearer token,
// returning the authenticated athlete's activities (most recent first).
func ListActivities(ctx context.Context, accessToken string, p ListActivitiesParams) ([]Activity, error) {
	q := url.Values{}
	if p.Before != nil {
		q.Set("before", strconv.FormatInt(p.Before.Unix(), 10))
	}
	if p.After != nil {
		q.Set("after", strconv.FormatInt(p.After.Unix(), 10))
	}
	if p.Page > 0 {
		q.Set("page", strconv.Itoa(p.Page))
	}
	perPage := p.PerPage
	if perPage <= 0 {
		perPage = 30
	}
	q.Set("per_page", strconv.Itoa(perPage))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/athlete/activities?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("strava request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("strava API %d: %s", resp.StatusCode, body)
	}

	var activities []Activity
	if err := json.NewDecoder(resp.Body).Decode(&activities); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	return activities, nil
}
