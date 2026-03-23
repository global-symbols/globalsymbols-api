package models

// Types mirror GLOBAL_SYMBOLS_API_SPEC.md

type CodingFramework struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Structure string `json:"structure"`
}

type Language struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Scope     string  `json:"scope"`
	Category  string  `json:"category"`
	ISO6391   *string `json:"iso639_1,omitempty"`
	ISO6392b  *string `json:"iso639_2b,omitempty"`
	ISO6392t  *string `json:"iso639_2t,omitempty"`
	ISO6393   string  `json:"iso639_3"`
}

type Licence struct {
	Name       string  `json:"name"`
	URL        *string `json:"url,omitempty"`
	Version    *string `json:"version,omitempty"`
	Properties *string `json:"properties,omitempty"`
}

type Symbolset struct {
	ID           int64    `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Publisher    string   `json:"publisher"`
	PublisherURL *string  `json:"publisher_url,omitempty"`
	Status       string   `json:"status"`
	Licence      Licence  `json:"licence"`
	Featured     *int64   `json:"featured_level,omitempty"`
	LogoURL      *string  `json:"logo_url,omitempty"`
}

type Picto struct {
	ID           int64      `json:"id"`
	SymbolsetID  int64      `json:"symbolset_id"`
	PartOfSpeech string     `json:"part_of_speech"`
	ImageURL     string     `json:"image_url"`
	NativeFormat string     `json:"native_format"`
	Adaptable    *bool      `json:"adaptable,omitempty"`
	Symbolset    *Symbolset `json:"symbolset,omitempty"`
}

type Label struct {
	ID              int64   `json:"id"`
	Text            string  `json:"text"`
	TextDiacritised *string `json:"text_diacritised,omitempty"`
	Description     *string `json:"description,omitempty"`
	Language        string  `json:"language"`
	Picto           Picto   `json:"picto"`
}

type Concept struct {
	ID            int64           `json:"id"`
	Subject       string          `json:"subject"`
	CodingFramework CodingFramework `json:"coding_framework"`
	Language      Language        `json:"language"`
	PictosCount   int64           `json:"pictos_count"`
	Pictos        []Picto         `json:"pictos"`
	APIURI        string          `json:"api_uri"`
	WWWURI        string          `json:"www_uri"`
}

type LabelSummary struct {
	Language       string  `json:"language"`
	Text           string  `json:"text"`
	TextDiacritised *string `json:"text_diacritised,omitempty"`
}

type PictoSummary struct {
	ID           int64          `json:"id"`
	PartOfSpeech string         `json:"part_of_speech"`
	ImageURL     string         `json:"image_url"`
	NativeFormat string         `json:"native_format"`
	Labels       []LabelSummary `json:"labels"`
}

type PagedPictosResponse struct {
	Items      []PictoSummary `json:"items"`
	Total      int64          `json:"total"`
	Deletions  []int64        `json:"deletions,omitempty"`
	LastUpdated *string        `json:"last_updated,omitempty"`
}

type User struct {
	ID               int64   `json:"id"`
	Prename          string  `json:"prename"`
	Surname          string  `json:"surname"`
	DefaultHairColor *string `json:"default_hair_colour,omitempty"`
	DefaultSkinColor *string `json:"default_skin_colour,omitempty"`
}

