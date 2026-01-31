package data

import "github.com/VJ-2303/code-runner/internal/validator"

type Filters struct {
	Language string
	PageSize int
	Page     int
}

func ValidateFilters(v *validator.Validator, f Filters) {
	if f.Language != "" {
		v.Check(validator.PermittedValue(f.Language, "ruby", "python", "javascript"), "language", "must be either ruby, python or javascript")
	}
	v.Check(f.Page >= 0, "page", "page must be greater than 0")
	v.Check(f.PageSize >= 3, "page_size", "page size must be greater than 2")
}

type Metadata struct {
	CurrentPage  int `json:"current_page,omitzero"`
	PageSize     int `json:"page_size,omitzero"`
	FirstPage    int `json:"first_page,omitzero"`
	LastPage     int `json:"last_page,omitzero"`
	TotalRecords int `json:"total_records,omitzero"`
}

func (f Filters) limit() int {
	return int(f.PageSize)
}

func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}

func CalculateMetaData(totalRecords, pageSize, page int) Metadata {
	return Metadata{
		CurrentPage:  page,
		FirstPage:    1,
		PageSize:     pageSize,
		LastPage:     (totalRecords + pageSize - 1) / pageSize,
		TotalRecords: totalRecords,
	}
}
