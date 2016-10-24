package mp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	LocationThanksgivingPoint = "683b08d3-6f8a-4501-a00f-a24601228dd6"
	LocationTheDistrict       = "24f371a5-1ad0-4ac5-b627-a24400a49818"
	LocationJordanCommons     = "9dafb9d0-ed8f-4a58-be62-a24b014cc0b4"
	LocationGeneva            = "83dd4871-c771-42a6-9177-a44a00e0ddd0"
)

func GetShortNameFromId(id string) string {
	switch id {
	case LocationThanksgivingPoint:
		return "thanksgivingpoint"
	case LocationTheDistrict:
		return "thedistrict"
	case LocationJordanCommons:
		return "jordancommons"
	case LocationGeneva:
		return "geneva"
	default:
		return "Unknown"
	}
}

func GetIdFromLocation(location string) string {
	switch location {
	case "Thanksgiving Point":
		return LocationThanksgivingPoint
	case "The District":
		return LocationTheDistrict
	case "Jordan Commons":
		return LocationJordanCommons
	case "Geneva":
		return LocationGeneva
	default:
		return "Unknown"
	}
}

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

type Theatre struct {
	HeroImage struct {
		Path   string `json:"filePath"`
		Height uint   `json:"height"`
		Width  int    `json:"width"`
	} `json:"HeroImage"`
	Id        string `json:"TheatreId"`
	Name      string `json:"name"`
	Street    string `json:"street"`
	City      string `json:"city"`
	State     string `json:"state"`
	Zip       string `json:"zip"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Phone     string `json:"phone"`
}

func GetTheatres() ([]Theatre, error) {
	ret := make([]Theatre, 0)

	resp, err := http.Get("https://beta.megaplextheatres.com/api/theatres/all")
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&ret)
	return ret, err
}

func GetTheatre(theatreId string) (Theatre, error) {
	ts := make([]Theatre, 0)

	resp, err := http.Get("https://beta.megaplextheatres.com/api/theatres/all")
	if err != nil {
		return Theatre{}, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&ts)
	for _, v := range ts {
		if v.Id == theatreId {
			return v, err
		}
	}
	return Theatre{}, errors.New("Not Found")
}

type Feature struct {
	FeatureCode    uint   `json:"featureCode"`
	Rating         string `json:"rating"`
	PrefeatureTime uint   `json:"prefeatureTime"`
	Runtime        uint   `json:"runtime"`
	Title          string `json:"studioTitle"`
	Synopsis       string `json:"synopsis"`
	Tagline        string `json:"tagline"`
	Website        string `json:"website"`

	Formats []string `json:"formats"`
	Genres  []string `json:"genres"`

	Backdrop struct {
		Small  string `json:"small"`
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"backdrop"`
	Banner struct {
		Small  string `json:"small"`
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"banner"`
	Logo struct {
		Small  string `json:"small"`
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"logo"`
	Poster struct {
		Small  string `json:"small"`
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"poster"`
	Trailer struct {
		Large struct {
			Codec     string `json:"codec"`
			ThumbPath string `json:"thumbPath"`
			FilePath  string `json:"filePath"`
		} `json:"large"`
	} `json:"trailers"`

	Performances []Performance `json:"performances"`
}

type Performance struct {
	Id                     uint      `json:"id"`
	AgeRestriction         uint      `json:"ageRestriction"`
	Amenities              []string  `json:"amenities"`
	Auditorium             string    `json:"auditorium"`
	AuditoriumFriendlyName string    `json:"auditoriumFriendlyName"`
	BusinessDate           string    `json:"businessDate"`
	DDDFlag                bool      `json:"dDDFlag"`
	DTSSoundFlag           bool      `json:"dTSSoundFlag"`
	DolbySoundFlag         bool      `json:"dolbySoundFlag"`
	FeatureCode            uint      `json:"featureCode"`
	FeatureTitle           string    `json:"-"`
	FeaturePoster          string    `json:"-"`
	Formats                []string  `json:"formats"`
	IMAXFlag               bool      `json:"imaxFlag"`
	ReservedSeating        bool      `json:"isReservedSeating"`
	Number                 int       `json:"number"`
	PassesAllowed          bool      `json:"passesAllowed"`
	SDDSSoundFlag          bool      `json:"sDDSSoundFlag"`
	Showtime               time.Time `json:"showTime"`
	Status                 string    `json:"status"`
	THXSoundFlag           bool      `json:"tHXSoundFlag"`
	VariableSeatPricing    bool      `json:"variableSeatPricing"`
}

type Schedule struct {
	AvailableDates   []string  `json:"availableDates"`
	AvailableFormats []string  `json:"availableFormats"`
	AvailableRatings []string  `json:"availableRatings"`
	Schedule         []Feature `json:"schedule"`
}

func GetSchedule(theatreId string) (Schedule, error) {
	ret := Schedule{}

	resp, err := http.Get("https://beta.megaplextheatres.com/api/theatres/schedule/" + theatreId)
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&ret)
	if err != nil {
		return ret, err
	}
	return ret, nil
}

func GetPerformances(theatreId string) ([]Performance, error) {
	s, err := GetSchedule(theatreId)
	if err != nil {
		return []Performance{}, err
	}
	ret := make([]Performance, 0)
	for _, f := range s.Schedule {
		for _, p := range f.Performances {
			p.FeatureCode = f.FeatureCode
			p.FeatureTitle = f.Title
			p.FeaturePoster = f.Poster.Large
			ret = append(ret, p)
		}
	}
	return ret, nil
}

func GetPerformancesForDay(theatreId string, date time.Time) ([]Performance, error) {
	ret := make([]Performance, 0)
	performances, err := GetPerformances(theatreId)
	if err != nil {
		return performances, err
	}
	ds := date.Local().Format("20060102")
	for _, p := range performances {
		if ds == p.BusinessDate {
			ret = append(ret, p)
		}
	}
	return ret, nil
}

type TicketTypes struct {
	TicketTypes []struct {
		Id                   string  `json:"id"`
		AgeRestricted        bool    `json:"ageRestricted"`
		Discount             bool    `json:"discountFlag"`
		FriendlyName         string  `json:"friendlyName"`
		OnHoldTicketType     bool    `json:"isOnHoldTicketType"`
		ReservedSeating      bool    `json:"isReservedSeating"`
		Name                 string  `json:"name"`
		Price                float64 `json:"price"`
		PricedTicketQty      uint    `json:"pricedTicketQty"`
		PricedTicketRequired bool    `json:"pricedTicketRequired"`
		Tax                  float64 `json:"tax"`
	} `json:"TicketTypes"`
	MaxHoldTickets   uint `json:"maxHoldTickets"`
	MaxOnHoldTickets uint `json:"maxOnHoldTickets"`
	MaxTickets       uint `json:"maxTickets"`
}

type SinglePerformance struct {
	Feature     Feature     `json:"feature"`
	Performance Performance `json:"performance"`
	Theatre     Theatre     `json:"theatre"`
	TicketTypes TicketTypes `json:"TicketTypes"`
}

func GetPerformance(performanceNumber string) (SinglePerformance, error) {
	ret := SinglePerformance{}

	resp, err := http.Get(fmt.Sprintf("https://beta.megaplextheatres.com/api/theatres/tickets/%s", performanceNumber))
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&ret)
	return ret, err
}

type Layout struct {
	SeatMessages []struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"seatMessages"`
	Seats []struct {
		Name   string `json:"name"`
		Row    int    `json:"row"`
		Column int    `json:"column"`
		Type   string `json:"type"`
	} `json:"seats"`
	TotalRowCount    int `json:"totalRowCount"`
	TotalColumnCount int `json:"totalColumnCount"`
	//Zones []
}

func GetLayout(performanceNumber string, theatreId string) (Layout, error) {
	ret := Layout{}

	resp, err := http.Get(fmt.Sprintf("https://beta.megaplextheatres.com/api/performances/seats/layout/%s/%s", performanceNumber, theatreId))
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&ret)
	return ret, err
}

type Preview struct {
	Id     string `json:"id"`
	Result struct {
		Code    int `json:"code"`
		SubCode int `json:"subCode"`
	} `json:"result"`
	SeatInfo struct {
		Overrides []struct {
			Row    int    `json:"row"`
			Column int    `json:"column"`
			Type   string `json:"type"`
		} `json:"overrides"`
		Statuses []struct {
			Row    int    `json:"row"`
			Column int    `json:"column"`
			Status string `json:"status"`
		} `json:"statuses"`
	} `json:"seatInfo"`
}

func GetPreview(performanceNumber string, theatreId string) (Preview, error) {
	ret := Preview{}

	resp, err := http.Get(fmt.Sprintf("https://beta.megaplextheatres.com/api/features/performances/seats/preview/%s/%s", performanceNumber, theatreId))
	if err != nil {
		return ret, err
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	err = d.Decode(&ret)
	return ret, err
}
