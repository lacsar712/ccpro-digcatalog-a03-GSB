package handlers

import (
	"net/http"
	"strconv"

	"digcatalog/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------- JoinGroups（残片拼合组） ----------

type joinGroupReq struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Note  string `json:"note"`
}

// attachOpenJoinGroups 为 finds 填充各自当前所属 open 拼合组（id/code），非持久化字段。
func (h *Handler) attachOpenJoinGroups(finds []models.Find) {
	ids := make([]uint, 0, len(finds))
	for _, f := range finds {
		ids = append(ids, f.ID)
	}
	if len(ids) == 0 {
		return
	}
	type row struct {
		FindID  uint
		GroupID uint
		Code    string
	}
	var rows []row
	if err := h.DB.Table("join_members").
		Select("join_members.find_id, join_groups.id AS group_id, join_groups.code").
		Joins("JOIN join_groups ON join_groups.id = join_members.group_id AND join_groups.deleted_at IS NULL").
		Where("join_members.find_id IN ? AND join_groups.status = ?", ids, models.JoinGroupStatusOpen).
		Scan(&rows).Error; err != nil {
		return
	}
	byFind := make(map[uint]row, len(rows))
	for _, r := range rows {
		byFind[r.FindID] = r
	}
	for i := range finds {
		if r, ok := byFind[finds[i].ID]; ok {
			gid := r.GroupID
			finds[i].JoinGroupID = &gid
			finds[i].JoinGroupCode = r.Code
		}
	}
}

func (h *Handler) preloadJoinGroup(group *models.JoinGroup) {
	h.DB.Preload("Members.Find.Unit.Site").Preload("Members.Find.Material").First(group, group.ID)
	var cnt int64
	h.DB.Model(&models.JoinMember{}).Where("group_id = ?", group.ID).Count(&cnt)
	group.MemberCount = cnt
}

// ListJoinGroups 列表，支持 ?status=open|closed 过滤。
func (h *Handler) ListJoinGroups(c *gin.Context) {
	q := h.DB.Order("id desc")
	if status := c.Query("status"); status != "" {
		if status != models.JoinGroupStatusOpen && status != models.JoinGroupStatusClosed {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status 仅支持 open 或 closed"})
			return
		}
		q = q.Where("status = ?", status)
	}
	var groups []models.JoinGroup
	if err := q.Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(groups) > 0 {
		ids := make([]uint, 0, len(groups))
		for _, g := range groups {
			ids = append(ids, g.ID)
		}
		type cntRow struct {
			GroupID uint
			Cnt     int64
		}
		var cnts []cntRow
		h.DB.Table("join_members").
			Select("group_id, count(*) AS cnt").
			Where("group_id IN ?", ids).
			Group("group_id").
			Scan(&cnts)
		byGroup := make(map[uint]int64, len(cnts))
		for _, r := range cnts {
			byGroup[r.GroupID] = r.Cnt
		}
		for i := range groups {
			groups[i].MemberCount = byGroup[groups[i].ID]
		}
	}
	c.JSON(http.StatusOK, groups)
}

func (h *Handler) GetJoinGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	h.preloadJoinGroup(&group)
	c.JSON(http.StatusOK, group)
}

func (h *Handler) CreateJoinGroup(c *gin.Context) {
	var req joinGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.Code == "" || req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "编号和名称必填"})
		return
	}
	var count int64
	h.DB.Model(&models.JoinGroup{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组编号已存在"})
		return
	}
	group := models.JoinGroup{
		Code:   req.Code,
		Title:  req.Title,
		Note:   req.Note,
		Status: models.JoinGroupStatusOpen,
	}
	if err := h.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "编号和名称必填"})
		return
	}
	var count int64
	h.DB.Model(&models.JoinGroup{}).Where("code = ? AND id <> ?", req.Code, group.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组编号已存在"})
		return
	}
	group.Code = req.Code
	group.Title = req.Title
	group.Note = req.Note
	if err := h.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.preloadJoinGroup(&group)
	c.JSON(http.StatusOK, group)
}

func (h *Handler) DeleteJoinGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		// 成员为关联表记录，物理删除；组本身软删除
		if err := tx.Where("group_id = ?", group.ID).Delete(&models.JoinMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.JoinGroup{}, group.ID).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// CloseJoinGroup 关闭拼合组：closed 后禁止再增删成员。
func (h *Handler) CloseJoinGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	if group.Status == models.JoinGroupStatusClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组已关闭"})
		return
	}
	group.Status = models.JoinGroupStatusClosed
	if err := h.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.preloadJoinGroup(&group)
	c.JSON(http.StatusOK, group)
}

type addMemberReq struct {
	FindID uint `json:"findId" binding:"required"`
}

// AddJoinMember 入组校验：组须 open；文物须存在且非“完整”；同一时刻只能属于一个 open 组。
func (h *Handler) AddJoinMember(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	if group.Status != models.JoinGroupStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组已关闭，禁止增删成员"})
		return
	}
	var req addMemberReq
	if err := c.ShouldBindJSON(&req); err != nil || req.FindID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "findId 必填"})
		return
	}
	var find models.Find
	if err := h.DB.First(&find, req.FindID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文物不存在"})
		return
	}
	if find.Completeness == "完整" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "完整器物无需拼合，不能加入拼合组"})
		return
	}
	var dup int64
	h.DB.Model(&models.JoinMember{}).Where("group_id = ? AND find_id = ?", group.ID, find.ID).Count(&dup)
	if dup > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该残片已在本组内"})
		return
	}
	var others []string
	h.DB.Table("join_members").
		Joins("JOIN join_groups ON join_groups.id = join_members.group_id AND join_groups.deleted_at IS NULL").
		Where("join_members.find_id = ? AND join_groups.status = ? AND join_members.group_id <> ?",
			find.ID, models.JoinGroupStatusOpen, group.ID).
		Pluck("join_groups.code", &others)
	if len(others) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该残片已属于进行中的拼合组 " + others[0]})
		return
	}
	member := models.JoinMember{GroupID: group.ID, FindID: find.ID}
	if err := h.DB.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.preloadJoinGroup(&group)
	c.JSON(http.StatusCreated, group)
}

func (h *Handler) RemoveJoinMember(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	findID, _ := strconv.Atoi(c.Param("findId"))
	var group models.JoinGroup
	if err := h.DB.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "拼合组不存在"})
		return
	}
	if group.Status != models.JoinGroupStatusOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拼合组已关闭，禁止增删成员"})
		return
	}
	res := h.DB.Where("group_id = ? AND find_id = ?", group.ID, findID).Delete(&models.JoinMember{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "该残片不在本组内"})
		return
	}
	h.preloadJoinGroup(&group)
	c.JSON(http.StatusOK, group)
}

// ListJoinGroupsByFind 按 findId 反查所属拼合组（含 open 与 closed）。
func (h *Handler) ListJoinGroupsByFind(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var find models.Find
	if err := h.DB.First(&find, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文物不存在"})
		return
	}
	var members []models.JoinMember
	if err := h.DB.Preload("Group").Where("find_id = ?", id).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	groups := make([]models.JoinGroup, 0, len(members))
	for _, m := range members {
		if m.Group != nil {
			groups = append(groups, *m.Group)
		}
	}
	c.JSON(http.StatusOK, groups)
}
