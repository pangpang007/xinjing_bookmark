package models

type LiteratureResponse struct {
	LiteratureText string `json:"literature_text"`
	BookName       string `json:"book_name"`
	Author         string `json:"author"`
	Style          string `json:"style"`
	Quota          *Quota `json:"quota,omitempty"`
}

type Quota struct {
	Used      int `json:"used"`
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
}

func NewQuota(used, limit int) *Quota {
	if limit <= 0 || used < 0 {
		return nil
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return &Quota{Used: used, Limit: limit, Remaining: remaining}
}

var DefaultLiterature = &LiteratureResponse{
	LiteratureText: "燕子去了，有再来的时候；杨柳枯了，有再青的时候；桃花谢了，有再开的时候。但是，聪明的，你告诉我，我们的日子为什么一去不复返呢？",
	BookName:       "匆匆",
	Author:         "朱自清",
	Style:          "nostalgic",
}

func NormalizeStyle(style string) string {
	switch style {
	case "warm", "melancholy", "nostalgic", "hopeful":
		return style
	default:
		return "nostalgic"
	}
}
