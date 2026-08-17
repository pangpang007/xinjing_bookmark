package models

import "time"

type User struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	OpenID    string    `json:"-" gorm:"column:openid;size:100;uniqueIndex;not null"`
	Nickname  string    `json:"nickname" gorm:"size:100;not null"`
	AvatarURL string    `json:"avatar_url" gorm:"column:avatar_url;size:500;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
