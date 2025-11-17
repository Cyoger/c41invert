package models

// ConvertRequest represents the parameters for image conversion
type ConvertRequest struct {
	SampleFraction       float64 `json:"sample_fraction,omitempty"`
	Lowlights           float64 `json:"lowlights,omitempty"`
	Highlights          float64 `json:"highlights,omitempty"`
	SCurve              bool    `json:"s_curve,omitempty"`
	OutputFormat        string  `json:"output_format,omitempty"`
	CenterMetering      bool    `json:"center_weighted_metering,omitempty"`
}

type ProblemDetail struct {
	Type     string `json:"type"`              
	Title    string `json:"title"`             
	Status   int    `json:"status"`            
	Detail   string `json:"detail,omitempty"`   
	Instance string `json:"instance,omitempty"`
}

type HealthResponse struct {
	Status string `json:"status"`
	Version string `json:"version"`
}