package model

type Category string

const (
	CategoryBills    Category = "bills"
	CategoryCar      Category = "car"
	CategoryHome     Category = "home"
	CategoryPhotos   Category = "photos"
	CategoryWork     Category = "work"
	CategoryPersonal Category = "personal"
	CategoryOther    Category = "other"
)

var AllCategories = []Category{
	CategoryBills,
	CategoryCar,
	CategoryHome,
	CategoryPhotos,
	CategoryWork,
	CategoryPersonal,
	CategoryOther,
}

func (c Category) IsValid() bool {
	for _, valid := range AllCategories {
		if c == valid {
			return true
		}
	}
	return false
}
