package client

import (
	"context"
	"net/url"
	"strings"
)

type runtimeReportResponse struct {
	apiEnvelope
	StartDate  string                `json:"startDate"`
	EndDate    string                `json:"endDate"`
	Columns    string                `json:"columns"`
	ReportList []runtimeReportEntry  `json:"reportList"`
	SensorList []runtimeSensorReport `json:"sensorList"`
}

type runtimeReportEntry struct {
	ThermostatIdentifier string   `json:"thermostatIdentifier"`
	RowList              []string `json:"rowList"`
}

type runtimeSensorReport struct {
	ThermostatIdentifier string                  `json:"thermostatIdentifier"`
	Sensors              []runtimeSensorMetadata `json:"sensors"`
	Columns              []string                `json:"columns"`
	Data                 []string                `json:"data"`
}

type runtimeSensorMetadata struct {
	SensorID    string `json:"sensorId"`
	SensorName  string `json:"sensorName"`
	SensorType  string `json:"sensorType"`
	SensorUsage string `json:"sensorUsage"`
}

type AirQualitySample struct {
	ThermostatID string
	SensorID     string
	SensorName   string
	Date         string
	Time         string
	Values       map[string]string
}

var airQualityCapabilities = map[string]bool{
	"airQuality":         true,
	"airQualityAccuracy": true,
	"airPressure":        true,
	"co2PPM":             true,
	"vocPPM":             true,
}

func (c *Client) GetAirQualityReport(ctx context.Context, startDate, endDate string) ([]AirQualitySample, error) {
	if err := c.EnsureThermostatID(ctx); err != nil {
		return nil, err
	}
	t, err := c.GetThermostat(ctx, "sensors")
	if err != nil {
		return nil, err
	}
	capabilityType := map[string]string{}
	sensorName := map[string]string{}
	for _, rs := range t.RemoteSensors {
		sensorName[rs.ID] = rs.Name
		for _, cap := range rs.Capability {
			capabilityType[rs.ID+":"+cap.ID] = cap.Type
		}
	}

	query := url.Values{}
	payload := map[string]any{
		"selection": map[string]any{
			"selectionType":  "thermostats",
			"selectionMatch": c.ThermostatID,
		},
		"startDate":      startDate,
		"endDate":        endDate,
		"columns":        "hvacMode",
		"includeSensors": true,
	}
	if err := addJSONQuery(query, "body", payload); err != nil {
		return nil, err
	}
	var resp runtimeReportResponse
	if err := c.do(ctx, "GET", "/1/runtimeReport", query, nil, &resp); err != nil {
		return nil, err
	}
	if err := checkStatus(resp.Status); err != nil {
		return nil, err
	}

	type colInfo struct {
		SensorID string
		CapType  string
	}
	var samples []AirQualitySample
	for _, report := range resp.SensorList {
		aqCols := map[int]colInfo{}
		for i, col := range report.Columns {
			if i < 2 {
				continue
			}
			idx := strings.LastIndex(col, ":")
			if idx < 0 {
				continue
			}
			sid, capID := col[:idx], col[idx+1:]
			capType, ok := capabilityType[sid+":"+capID]
			if !ok || !airQualityCapabilities[capType] {
				continue
			}
			aqCols[i] = colInfo{SensorID: sid, CapType: capType}
		}
		if len(aqCols) == 0 {
			continue
		}
		for _, row := range report.Data {
			fields := strings.Split(row, ",")
			if len(fields) < 2 {
				continue
			}
			bySensor := map[string]*AirQualitySample{}
			for i, info := range aqCols {
				if i >= len(fields) {
					continue
				}
				val := strings.TrimSpace(fields[i])
				if val == "" {
					continue
				}
				s, ok := bySensor[info.SensorID]
				if !ok {
					s = &AirQualitySample{
						ThermostatID: report.ThermostatIdentifier,
						SensorID:     info.SensorID,
						SensorName:   sensorName[info.SensorID],
						Date:         fields[0],
						Time:         fields[1],
						Values:       map[string]string{},
					}
					bySensor[info.SensorID] = s
				}
				s.Values[info.CapType] = val
			}
			for _, s := range bySensor {
				samples = append(samples, *s)
			}
		}
	}
	return samples, nil
}
