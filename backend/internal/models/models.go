package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Username     string         `json:"username" gorm:"uniqueIndex;size:64;not null"`
	PasswordHash string         `json:"-" gorm:"size:255;not null"`
	Role         string         `json:"role" gorm:"size:32;not null"` // admin | recorder
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

type Site struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:128;not null"`
	Period    string         `json:"period" gorm:"size:64;not null"` // 新石器/商周等
	Latitude  float64        `json:"latitude"`
	Longitude float64        `json:"longitude"`
	Manager   string         `json:"manager" gorm:"size:64"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	Units     []Unit         `json:"units,omitempty" gorm:"foreignKey:SiteID"`
}

type Unit struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	SiteID           uint           `json:"siteId" gorm:"not null;index"`
	Code             string         `json:"code" gorm:"size:64;not null"` // T1, T2...
	DepthMin         float64        `json:"depthMin"`
	DepthMax         float64        `json:"depthMax"`
	StratumDesc      string         `json:"stratumDesc" gorm:"type:text"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
	Site             *Site          `json:"site,omitempty" gorm:"foreignKey:SiteID"`
	Finds            []Find         `json:"finds,omitempty" gorm:"foreignKey:UnitID"`
}

type Material struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"uniqueIndex;size:64;not null"`
	Description string         `json:"description" gorm:"type:text"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type Find struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	UnitID       uint           `json:"unitId" gorm:"not null;index"`
	MaterialID   *uint          `json:"materialId" gorm:"index"`
	RegisterNo   string         `json:"registerNo" gorm:"uniqueIndex;size:64;not null"`
	ArtifactType string         `json:"artifactType" gorm:"size:64;not null"` // 陶片/青铜器/骨器
	MaterialName string         `json:"materialName" gorm:"size:64"`          // 冗余展示字段
	Completeness string         `json:"completeness" gorm:"size:32"`          // 完整/残缺/碎片
	FindDate     *time.Time     `json:"findDate" gorm:"type:date"`
	Description  string         `json:"description" gorm:"type:text"`
	StorageLoc   string         `json:"storageLoc" gorm:"size:128"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
	Unit         *Unit          `json:"unit,omitempty" gorm:"foreignKey:UnitID"`
	Material     *Material      `json:"material,omitempty" gorm:"foreignKey:MaterialID"`

	// JoinGroupCode 为只读展示字段（不入库、不加列）：
	// 拼合关系完全由 join_members 关联表表达，文物主表仍只有 finds。
	JoinGroupCode string `json:"joinGroupCode,omitempty" gorm:"-"`
}

// JoinGroup 残片拼合组。拼合关系通过独立的 JoinMember 关联表表达，
// 不另造平行文物主表，也不在 Find 上挂文本字段冒充拼合。
type JoinGroup struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Code      string         `json:"code" gorm:"uniqueIndex;size:64;not null"` // 全库唯一
	Status    string         `json:"status" gorm:"size:16;not null;index"`     // open | closed
	Title     string         `json:"title" gorm:"size:128;not null"`
	Note      string         `json:"note" gorm:"type:text"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	Members   []JoinMember   `json:"members,omitempty" gorm:"foreignKey:GroupID"`

	// MemberCount 只读统计字段（不入库），列表接口填充。
	MemberCount int64 `json:"memberCount,omitempty" gorm:"-"`
}

// JoinMember 拼合组成员：(group_id, find_id) 全库唯一。
type JoinMember struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	GroupID   uint       `json:"groupId" gorm:"not null;uniqueIndex:idx_join_member_group_find"`
	FindID    uint       `json:"findId" gorm:"not null;uniqueIndex:idx_join_member_group_find"`
	Note      string     `json:"note" gorm:"type:text"`
	CreatedAt time.Time  `json:"createdAt"`
	Group     *JoinGroup `json:"group,omitempty" gorm:"foreignKey:GroupID"`
	Find      *Find      `json:"find,omitempty" gorm:"foreignKey:FindID"`
}
