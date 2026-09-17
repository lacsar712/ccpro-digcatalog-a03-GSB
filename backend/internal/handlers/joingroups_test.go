package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"digcatalog/internal/models"
	"digcatalog/internal/seed"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTest(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// 每个用例独立的内存库，避免相互污染
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&models.User{}, &models.Site{}, &models.Unit{}, &models.Material{},
		&models.Find{}, &models.JoinGroup{}, &models.JoinMember{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	h := New(db, "test-secret")
	r := gin.New()
	api := r.Group("/api")
	api.GET("/finds", h.ListFinds)
	api.DELETE("/finds/:id", h.DeleteFind)
	api.GET("/finds/:id/joingroups", h.ListJoinGroupsByFind)
	api.GET("/joingroups", h.ListJoinGroups)
	api.GET("/joingroups/:id", h.GetJoinGroup)
	api.POST("/joingroups", h.CreateJoinGroup)
	api.PUT("/joingroups/:id", h.UpdateJoinGroup)
	api.DELETE("/joingroups/:id", h.DeleteJoinGroup)
	api.POST("/joingroups/:id/close", h.CloseJoinGroup)
	api.POST("/joingroups/:id/members", h.AddJoinMember)
	api.DELETE("/joingroups/:id/members/:findId", h.RemoveJoinMember)
	return db, r
}

func doReq(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var parsed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	return w, parsed
}

func doReqList[T any](t *testing.T, r *gin.Engine, path string) []T {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s -> %d: %s", path, w.Code, w.Body.String())
	}
	var out []T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return out
}

// seedFixtures 造一个探方 + 三件文物（完整/残缺/碎片各一）
func seedFixtures(t *testing.T, db *gorm.DB) (complete, broken, fragment models.Find) {
	t.Helper()
	site := models.Site{Name: "测试遗址", Period: "商周"}
	db.Create(&site)
	unit := models.Unit{SiteID: site.ID, Code: "T1"}
	db.Create(&unit)
	complete = models.Find{UnitID: unit.ID, RegisterNo: "T-001", ArtifactType: "陶片", Completeness: "完整"}
	broken = models.Find{UnitID: unit.ID, RegisterNo: "T-002", ArtifactType: "陶片", Completeness: "残缺"}
	fragment = models.Find{UnitID: unit.ID, RegisterNo: "T-003", ArtifactType: "陶片", Completeness: "碎片"}
	db.Create(&complete)
	db.Create(&broken)
	db.Create(&fragment)
	return
}

func createGroup(t *testing.T, r *gin.Engine, code string) models.JoinGroup {
	t.Helper()
	w, body := doReq(t, r, http.MethodPost, "/api/joingroups",
		map[string]any{"code": code, "title": code + " 拼合", "note": "备注"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create group %s -> %d: %v", code, w.Code, body)
	}
	var g models.JoinGroup
	b, _ := json.Marshal(body)
	_ = json.Unmarshal(b, &g)
	return g
}

func TestJoinGroupCreateAndUniqueCode(t *testing.T) {
	_, r := setupTest(t)
	g := createGroup(t, r, "JG-001")
	if g.Status != models.JoinGroupStatusOpen {
		t.Fatalf("new group status = %q, want open", g.Status)
	}
	// code 全库唯一
	w, body := doReq(t, r, http.MethodPost, "/api/joingroups",
		map[string]any{"code": "JG-001", "title": "重复编号"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate code -> %d, want 400 (%v)", w.Code, body)
	}
	// 缺字段
	w, _ = doReq(t, r, http.MethodPost, "/api/joingroups", map[string]any{"code": "JG-002"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing title -> %d, want 400", w.Code)
	}
}

func TestJoinGroupListByStatus(t *testing.T) {
	_, r := setupTest(t)
	createGroup(t, r, "JG-001")
	g2 := createGroup(t, r, "JG-002")
	w, _ := doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/close", g2.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("close -> %d", w.Code)
	}

	open := doReqList[models.JoinGroup](t, r, "/api/joingroups?status=open")
	if len(open) != 1 || open[0].Code != "JG-001" {
		t.Fatalf("open list = %+v", open)
	}
	closed := doReqList[models.JoinGroup](t, r, "/api/joingroups?status=closed")
	if len(closed) != 1 || closed[0].Code != "JG-002" {
		t.Fatalf("closed list = %+v", closed)
	}
	all := doReqList[models.JoinGroup](t, r, "/api/joingroups")
	if len(all) != 2 {
		t.Fatalf("all list len = %d", len(all))
	}
	// 非法 status
	req := httptest.NewRequest(http.MethodGet, "/api/joingroups?status=bad", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("bad status -> %d, want 400", w2.Code)
	}
}

func TestAddMemberRules(t *testing.T) {
	db, r := setupTest(t)
	complete, broken, fragment := seedFixtures(t, db)
	g := createGroup(t, r, "JG-001")

	// 完整器物入组 -> 400
	w, body := doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": complete.ID})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("add complete find -> %d, want 400 (%v)", w.Code, body)
	}

	// 正常入组
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})
	if w.Code != http.StatusCreated {
		t.Fatalf("add broken -> %d", w.Code)
	}

	// 重复入组 -> 400
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("re-add -> %d, want 400", w.Code)
	}

	// 同一时刻只能属于一个 open 组
	g2 := createGroup(t, r, "JG-002")
	w, body = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g2.ID),
		map[string]any{"findId": broken.ID})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("add to second open group -> %d, want 400 (%v)", w.Code, body)
	}
	// 另一件残片可以进第二组
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g2.ID),
		map[string]any{"findId": fragment.ID})
	if w.Code != http.StatusCreated {
		t.Fatalf("add fragment to g2 -> %d", w.Code)
	}

	// 不存在的文物 -> 400
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": 9999})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("add missing find -> %d, want 400", w.Code)
	}
}

func TestClosedGroupForbidsMemberChanges(t *testing.T) {
	db, r := setupTest(t)
	_, broken, _ := seedFixtures(t, db)
	g := createGroup(t, r, "JG-001")
	doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})

	// 关闭组
	w, body := doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/close", g.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("close -> %d (%v)", w.Code, body)
	}
	// 重复关闭 -> 400
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/close", g.ID), nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("re-close -> %d, want 400", w.Code)
	}
	// closed 组禁止增成员
	_, _, fragment := seedFixtures(t, db)
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": fragment.ID})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("add to closed -> %d, want 400", w.Code)
	}
	// closed 组禁止删成员
	w, _ = doReq(t, r, http.MethodDelete,
		fmt.Sprintf("/api/joingroups/%d/members/%d", g.ID, broken.ID), nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("remove from closed -> %d, want 400", w.Code)
	}
}

func TestRemoveMemberAndRejoin(t *testing.T) {
	db, r := setupTest(t)
	_, broken, _ := seedFixtures(t, db)
	g := createGroup(t, r, "JG-001")
	doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})

	w, _ := doReq(t, r, http.MethodDelete, fmt.Sprintf("/api/joingroups/%d/members/%d", g.ID, broken.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("remove -> %d", w.Code)
	}
	// 再移出 -> 404
	w, _ = doReq(t, r, http.MethodDelete, fmt.Sprintf("/api/joingroups/%d/members/%d", g.ID, broken.ID), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("remove again -> %d, want 404", w.Code)
	}
	// 移出后可重新入组（关联表物理删除，唯一约束不残留）
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})
	if w.Code != http.StatusCreated {
		t.Fatalf("rejoin -> %d", w.Code)
	}
}

func TestReverseLookupAndFindsListEnrichment(t *testing.T) {
	db, r := setupTest(t)
	_, broken, _ := seedFixtures(t, db)
	g := createGroup(t, r, "JG-001")
	doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})

	// 按 findId 反查所属组
	groups := doReqList[models.JoinGroup](t, r, fmt.Sprintf("/api/finds/%d/joingroups", broken.ID))
	if len(groups) != 1 || groups[0].Code != "JG-001" {
		t.Fatalf("reverse lookup = %+v", groups)
	}

	// finds 列表附带 open 组 code
	type findView struct {
		ID            uint   `json:"id"`
		JoinGroupCode string `json:"joinGroupCode"`
	}
	finds := doReqList[findView](t, r, "/api/finds")
	var seen bool
	for _, f := range finds {
		if f.ID == broken.ID {
			seen = true
			if f.JoinGroupCode != "JG-001" {
				t.Fatalf("find %d joinGroupCode = %q", f.ID, f.JoinGroupCode)
			}
		} else if f.JoinGroupCode != "" {
			t.Fatalf("find %d unexpected joinGroupCode %q", f.ID, f.JoinGroupCode)
		}
	}
	if !seen {
		t.Fatal("broken find not in list")
	}
}

func TestDeleteGroupRemovesMembers(t *testing.T) {
	db, r := setupTest(t)
	_, broken, _ := seedFixtures(t, db)
	g := createGroup(t, r, "JG-001")
	doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})

	w, _ := doReq(t, r, http.MethodDelete, fmt.Sprintf("/api/joingroups/%d", g.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete group -> %d", w.Code)
	}
	var cnt int64
	db.Model(&models.JoinMember{}).Where("group_id = ?", g.ID).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("members left after delete: %d", cnt)
	}
	// 删除组后该残片可加入新 open 组
	g2 := createGroup(t, r, "JG-002")
	w, _ = doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g2.ID),
		map[string]any{"findId": broken.ID})
	if w.Code != http.StatusCreated {
		t.Fatalf("rejoin after group delete -> %d", w.Code)
	}
}

func TestDeleteFindCleansMembership(t *testing.T) {
	db, r := setupTest(t)
	_, broken, _ := seedFixtures(t, db)
	g := createGroup(t, r, "JG-001")
	doReq(t, r, http.MethodPost, fmt.Sprintf("/api/joingroups/%d/members", g.ID),
		map[string]any{"findId": broken.ID})

	w, _ := doReq(t, r, http.MethodDelete, fmt.Sprintf("/api/finds/%d", broken.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete find -> %d", w.Code)
	}
	var cnt int64
	db.Model(&models.JoinMember{}).Where("find_id = ?", broken.ID).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("membership left after find delete: %d", cnt)
	}
}

func TestUpdateJoinGroup(t *testing.T) {
	_, r := setupTest(t)
	g1 := createGroup(t, r, "JG-001")
	createGroup(t, r, "JG-002")

	w, body := doReq(t, r, http.MethodPut, fmt.Sprintf("/api/joingroups/%d", g1.ID),
		map[string]any{"code": "JG-001A", "title": "改名", "note": "新备注"})
	if w.Code != http.StatusOK {
		t.Fatalf("update -> %d (%v)", w.Code, body)
	}
	if body["code"] != "JG-001A" || body["title"] != "改名" {
		t.Fatalf("update result = %v", body)
	}
	// 改成别人的 code -> 400
	w, _ = doReq(t, r, http.MethodPut, fmt.Sprintf("/api/joingroups/%d", g1.ID),
		map[string]any{"code": "JG-002", "title": "冲突"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("update to dup code -> %d, want 400", w.Code)
	}
}

// TestSeedJoinGroups 验证种子数据：≥2 open 组、≥1 closed 组，且成员关系落库
func TestSeedJoinGroups(t *testing.T) {
	db, _ := setupTest(t)
	seed.Run(db)

	var openCnt, closedCnt int64
	db.Model(&models.JoinGroup{}).Where("status = ?", models.JoinGroupStatusOpen).Count(&openCnt)
	db.Model(&models.JoinGroup{}).Where("status = ?", models.JoinGroupStatusClosed).Count(&closedCnt)
	if openCnt < 2 {
		t.Fatalf("seed open groups = %d, want >= 2", openCnt)
	}
	if closedCnt < 1 {
		t.Fatalf("seed closed groups = %d, want >= 1", closedCnt)
	}
	var memberCnt int64
	db.Model(&models.JoinMember{}).Count(&memberCnt)
	if memberCnt == 0 {
		t.Fatal("seed has no join members")
	}
	// 种子成员不得包含「完整」器物
	var bad int64
	db.Table("join_members").
		Joins("JOIN finds ON finds.id = join_members.find_id").
		Where("finds.completeness = ?", "完整").
		Count(&bad)
	if bad != 0 {
		t.Fatalf("seed has %d complete finds in join groups", bad)
	}
}
