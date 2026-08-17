package models

import "time"

type History struct {
	ID             int64     `json:"id" gorm:"primaryKey"`
	UserID         int64     `json:"user_id" gorm:"index:idx_history_user_id;not null"`
	MoodInput      string    `json:"mood_input" gorm:"type:text;not null"`
	LiteratureText string    `json:"literature_text" gorm:"type:text;not null"`
	BookName       string    `json:"book_name" gorm:"size:200;not null"`
	Author         string    `json:"author" gorm:"size:100;not null"`
	Style          string    `json:"style" gorm:"size:20;not null"`
	ImageURL       string    `json:"image_url" gorm:"column:image_url;size:500"`
	CreatedAt      time.Time `json:"created_at" gorm:"index:idx_history_created_at"`
}

func (History) TableName() string {
	return "history"
}
