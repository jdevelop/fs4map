package kmlapi

type HasId struct {
	Id string `json:"id"`
}

type HasName struct {
	Name string `json:"name"`
}

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Category struct {
	HasId
	HasName
}

type Venue struct {
	HasId
	HasName
	Location        Location   `json:"location"`
	Categories      []Category `json:"categories"`
	VisitTimestamps []int64    `json:"-"`
}

type GlobalCategory struct {
	HasId
	HasName
	Children []GlobalCategory `json:"categories"`
}

type fsqCategory struct {
	Response struct {
		Categories []GlobalCategory `json:"categories"`
	} `json:"response"`
}
