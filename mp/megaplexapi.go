package mp

import (
	"encoding/json"
	"net/http"
	"time"
)

const (
	LocationThanksgivingPoint = "683b08d3-6f8a-4501-a00f-a24601228dd6"
	LocationTheDistrict       = "24f371a5-1ad0-4ac5-b627-a24400a49818"
	LocationJordanCommons     = "9dafb9d0-ed8f-4a58-be62-a24b014cc0b4"
	LocationGeneva            = "83dd4871-c771-42a6-9177-a44a00e0ddd0"
)

func GetLocationFromId(id string) string {
	switch id {
	case LocationThanksgivingPoint:
		return "Thanksgiving Point"
	case LocationTheDistrict:
		return "The District"
	case LocationJordanCommons:
		return "Jordan Commons"
	case LocationGeneva:
		return "Geneva"
	default:
		return "Unknown"
	}
}

func GetAddressFromId(id string) string {
	switch id {
	case LocationThanksgivingPoint:
		return "2935 Thanksgiving Way, Lehi, UT 84043"
	case LocationTheDistrict:
		return "3761 W Parkway Plaza Dr, South Jordan, UT, 84095"
	case LocationJordanCommons:
		return "9335 State Street, Sandy, UT, 84070"
	case LocationGeneva:
		return "600 North Mill Road, Vineyard, UT, 84058"
	default:
		return "Unknown"
	}
}

const (
	//id:3
	FormatIMAX = "IMAX"
	//id:4
	FormatDBox = "D-BOX"
)

const (
	//id:7
	AmenityLuxury = "Luxury"
)

type Movie struct {
	FeatureCode int `json:"featureCode"`
	Poster      struct {
		Small  string `json:"small"`
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"poster"`
	Rating   string `json:"rating"`
	Runtime  uint   `json:"runtime"`
	Title    string `json:"studioTitle"`
	Synopsis string `json:"synopsis"`
	Tagline  string `json:"tagline"`
	Trailer  struct {
		Large struct {
			Codec     string `json:"codec"`
			ThumbPath string `json:"thumbPath"`
			FilePath  string `json:"filePath"`
		} `json:"large"`
	} `json:"trailers"`
	Website string `json:"website"`

	Performances []Performance `json:"performances"`
}

type Performance struct {
	AgeRestriction int `json:"ageRestriction"`
	Amenities      []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"amenities"`
	Auditorium             string `json:"auditorium"`
	AuditoriumFriendlyName string `json:"auditoriumFriendlyName"`
	BusinessDate           string `json:"businessDate"`
	DDDFlag                bool   `json:"dDDFlag"`
	DTSSoundFlag           bool   `json:"dTSSoundFlag"`
	DolbySoundFlag         bool   `json:"dolbySoundFlag"`
	FeatureCode            int    `json:"featureCode"`
	FeatureTitle           string `json:"-"`
	FeaturePoster          string `json:"-"`
	Formats                []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"formats"`
	IMAXFlag            bool      `json:"imaxFlag"`
	IsReservedSeating   bool      `json:"isReservedSeating"`
	Number              int       `json:"number"`
	PassesAllowed       bool      `json:"passesAllowed"`
	SDDSSoundFlag       bool      `json:"sDDSSoundFlag"`
	Showtime            time.Time `json:"showTime"`
	Status              string    `json:"status"`
	THXSoundFlag        bool      `json:"tHXSoundFlag"`
	VariableSeatPricing bool      `json:"variableSeatPricing"`
}

func GetPerformances(theatreId string, date time.Time) ([]Performance, error) {
	ret := make([]Performance, 0)
	var w = struct {
		Schedule map[string][]Movie `json:"schedule"`
	}{}

	resp, err := http.Get("https://beta.megaplextheatres.com/api/theatres/schedule?theatreId=" + theatreId)
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&w)
	if err != nil {
		return ret, err
	}
	for _, m := range w.Schedule[date.Local().Format("20060102")] {
		for _, p := range m.Performances {
			p.FeatureTitle = m.Title
			p.FeaturePoster = m.Poster.Large
			ret = append(ret, p)
		}
	}
	return ret, nil

}
