package portevent

type Feed struct {
	Cursor string  `json:"cursor"`
	Events []Event `json:"events"`
}
