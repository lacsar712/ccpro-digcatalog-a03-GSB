package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"digcatalog/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------- JoinGroup（残片拼合组） ----------

type joinGroupReq struct {
	Code   string `json:"code"`
	Title  string `json:"title"`
	Note   string `json:"note"`
	Status string `json:"status"`
}

type joinMemberReq struct {
	FindID uint   `json:"findId"`
	Note   string `json:"note"`
}

// fillJoinGroupCodes 为文物列表填充所属拼合组 code（只读展示字段，不入库）。
// 同一文物理论上可能留有已关闭组的历史记录，优先取其当前 open 组。
func fillJoinGroupCodes(db *gorm.DB, finds []models.Find) {
	if len(finds) == 0 {
		return
	}
	ids := make([]uint, len(finds))
	for i := range finds {
		ids[i] = finds[i].ID
	}

	type row struct {
		FindID uint
		Code   string
		Status string
	}
	var rows []row
	db.Table("join_members AS m").
		Select("m.find_id AS find_id, g.code AS code, g.status AS status").
		Joins("JOIN join_groups AS g ON g.id = m.group_id AND g.deleted_at IS NULL").
		Where("m.find_id IN ?", ids).
		Scan(&rows)

	codeByFind := make(map[uint]string, len(rows))
	for _, r := range rows {
		if r.Status == "open" {
			codeByFind[r.FindID] = r.Code // open 优先
		} else if _, ok := codeByFind[r.FindID]; !ok {
			codeByFind[r.FindID] = r.Code
		}
	}
	for i := range finds {
		finds[i].JoinGroupCode = codeByFind[finds[i].ID]
	}
}

func preloadGroup(db *gorm.DB, group *models.JoinGroup) error {
	return db.Preload("Members").
		Preload("Members.Find").
		Preload("Members.Find.Unit").
		Preload("Members.Find.Material").
		First(group, group.ID).Error
}

// ListJoinGroups GET /api/join-groups?status=open|closed&findId=12
func (h *Handler) ListJoinGroups(c *gin.Context) {
	q := h.DB.Model(&models.JoinGroup{}).Order("id desc")
	if status := c.Query("status"); status != "" {
		if status != "open" && status != "closed" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status 只能为 open 或 closed"})
			return
		}
		q = q.Where("status = ?", status)
	}
	if findID := c.Query("findId"); findID != "" {
		q = q.Where("id IN (?)",
			h.DB.Model(&models.JoinMember{}).Select("group_id").Where("find_id = ?", findID))
	}

	var groups []models.JoinGroup
	if err := q.Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(groups) > 0 {
		ids := make([]uint, len(groups))
		for i := range groups {
			ids[i] = groups[i].ID
		}
		type cnt struct {
			GroupID uint
			N       int64
		}
		var counts []cnt
		h.DB.Model(&models.JoinMember{}).
			Select("group_id AS group_id, count(*) AS n").
			Where("group_id IN ?", ids).
			Group("group_id").
			Scan(&counts)
		countByID := make(map[uint]int64, len(counts))
		for _, ct := range counts {
			countByID[ct.GroupID] = ct.N
		}
		for i := range groups {
			groups[i].MemberCount = countByID[groups[i].ID]
		}
	}
	c.JSON(http.StatusOK, groups)
}

// GetJoinGroup GET /api/join-groups/:id
func (h *Handler) GetJoinGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	if err := preloadGroup(h.DB, &group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

// CreateJoinGroup POST /api/join-groups（新建组一律为 open）
func (h *Handler) CreateJoinGroup(c *gin.Context) {
	var req joinGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.Code == "" || req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组编号与标题必填"})
		return
	}
	if err := h.checkCodeUnique(req.Code, 0); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	group := models.JoinGroup{
		Code:   req.Code,
		Title:  req.Title,
		Note:   req.Note,
		Status: "open",
	}
	if err := h.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

// UpdateJoinGroup PUT /api/join-groups/:id（仅可改 code/title/note，状态由关闭接口流转）
func (h *Handler) UpdateJoinGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	var req joinGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.Code == "" || req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组编号与标题必填"})
		return
	}
	if err := h.checkCodeUnique(req.Code, group.ID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	group.Code = req.Code
	group.Title = req.Title
	group.Note = req.Note
	if err := h.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, group)
}

// DeleteJoinGroup DELETE /api/join-groups/:id（连带删除成员关联，文物主数据不动）
func (h *Handler) DeleteJoinGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&models.JoinMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.JoinGroup{}, id).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// CloseJoinGroup POST /api/join-groups/:id/close
func (h *Handler) CloseJoinGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	if group.Status == "closed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组已关闭，无需重复操作"})
		return
	}
	group.Status = "closed"
	if err := h.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = preloadGroup(h.DB, &group)
	c.JSON(http.StatusOK, group)
}

// AddJoinMember POST /api/join-groups/:id/members  body: {"findId": 12}
func (h *Handler) AddJoinMember(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req joinMemberReq
	if err := c.ShouldBindJSON(&req); err != nil || req.FindID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "findId 必填"})
		return
	}

	var group models.JoinGroup
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&group, id).Error; err != nil {
			return err
		}
		if group.Status == "closed" {
			return errJoinClosed
		}

		var find models.Find
		if err := tx.First(&find, req.FindID).Error; err != nil {
			return errFindNotFound
		}
		if find.Completeness == "完整" {
			return errCompleteFind
		}

		// 已在本组 → 幂等报错
		var n int64
		tx.Model(&models.JoinMember{}).
			Where("group_id = ? AND find_id = ?", group.ID, req.FindID).Count(&n)
		if n > 0 {
			return errAlreadyMember
		}

		// 同一时刻一个 Find 只能属于一个 open 组
		var others []models.JoinMember
		if err := tx.Preload("Group").
			Where("find_id = ? AND group_id <> ?", req.FindID, group.ID).
			Find(&others).Error; err != nil {
			return err
		}
		for _, mm := range others {
			if mm.Group != nil && mm.Group.Status == "open" {
				return errInOtherOpenGroup
			}
		}

		member := models.JoinMember{GroupID: group.ID, FindID: req.FindID, Note: req.Note}
		return tx.Create(&member).Error
	})

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	case errors.Is(err, errJoinClosed):
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组已关闭，禁止增删成员"})
		return
	case errors.Is(err, errFindNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "文物不存在"})
		return
	case errors.Is(err, errCompleteFind):
		c.JSON(http.StatusBadRequest, gin.H{"error": "完整器物不能加入拼合组（仅残缺/碎片可拼合）"})
		return
	case errors.Is(err, errAlreadyMember):
		c.JSON(http.StatusBadRequest, gin.H{"error": "该文物已在此拼合组中"})
		return
	case errors.Is(err, errInOtherOpenGroup):
		c.JSON(http.StatusBadRequest, gin.H{"error": "该文物已属于另一个 open 拼合组，须先移出或关闭原组"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := preloadGroup(h.DB, &group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

// RemoveJoinMember DELETE /api/join-groups/:id/members/:findId
func (h *Handler) RemoveJoinMember(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	findID, _ := strconv.Atoi(c.Param("findId"))

	var group models.JoinGroup
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&group, id).Error; err != nil {
			return err
		}
		if group.Status == "closed" {
			return errJoinClosed
		}
		res := tx.Where("group_id = ? AND find_id = ?", id, findID).Delete(&models.JoinMember{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errNotMember
		}
		return nil
	})
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	case errors.Is(err, errJoinClosed):
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组已关闭，禁止增删成员"})
		return
	case errors.Is(err, errNotMember):
		c.JSON(http.StatusNotFound, gin.H{"error": "该文物不在此拼合组中"})
		return
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = preloadGroup(h.DB, &group)
	c.JSON(http.StatusOK, group)
}

func (h *Handler) checkCodeUnique(code string, excludeID uint) error {
	var n int64
	h.DB.Model(&models.JoinGroup{}).
		Where("code = ? AND id <> ?", code, excludeID).Count(&n)
	if n > 0 {
		return errors.New("拼合组编号已存在，code 须全库唯一")
	}
	return nil
}

var (
	errJoinClosed       = errors.New("join group closed")
	errFindNotFound     = errors.New("find not found")
	errCompleteFind     = errors.New("find is complete")
	errAlreadyMember    = errors.New("find already a member")
	errNotMember        = errors.New("find is not a member")
	errInOtherOpenGroup = errors.New("find already in another open group")
)
