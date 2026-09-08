package handler

import (
	"api/service"
	"api/utils/constants"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type CalendarHandler struct{ db *gorm.DB }

var errSourceEvent = errors.New("source event cannot be edited")
var errCustomKey = errors.New("custom event key reused with a different body")

func NewCalendarHandler(db *gorm.DB) *CalendarHandler { return &CalendarHandler{db} }

// Every personal operation uses the server session; user_id is never accepted.
func personalUser(c *gin.Context) (uint, bool) {
	id := c.GetUint(constants.ContextUserID)
	if id == 0 {
		c.JSON(401, gin.H{"message": "Войдите в аккаунт"})
		return 0, false
	}
	return id, true
}
func personalJSON(c *gin.Context, value any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		c.JSON(400, gin.H{"message": "Некорректные поля запроса"})
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		c.JSON(400, gin.H{"message": "Ожидался один JSON объект"})
		return false
	}
	return true
}
func planID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 63)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "Некорректный ID"})
		return 0, false
	}
	return id, true
}
func calendarError(c *gin.Context, err error) {
	if errors.Is(err, errCustomKey) {
		c.JSON(409, gin.H{"message": "Это событие уже создано. Откройте его в плане, чтобы изменить"})
		return
	}
	var pgError *pgconn.PgError
	if errors.Is(err, errSourceEvent) {
		c.JSON(400, gin.H{"message": "Даты события из источника нельзя менять; создайте своё событие"})
		return
	}
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		c.JSON(409, gin.H{"message": "Такая запись уже существует"})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"message": "Запись не найдена"})
	} else {
		c.JSON(500, gin.H{"message": "Не удалось выполнить запрос"})
	}
}

func (h *CalendarHandler) Events(c *gin.Context) {
	if k := c.Query("kind"); k != "" && !service.CalendarKind(k) {
		c.JSON(400, gin.H{"message": "Неизвестный вид события"})
		return
	}
	q := h.db.WithContext(c.Request.Context()).Table("olympguide.calendar_event").Where("active=true")
	if k := c.Query("kind"); k != "" {
		q = q.Where("payload->>'kind'=?", k)
	}
	if s := c.Query("season"); s != "" {
		q = q.Where("season=?", s)
	}
	if k := c.Query("olympiad_key"); k != "" {
		q = q.Where("jsonb_exists(payload->'olympiad_keys',?)", k)
	}
	if k := c.Query("program_key"); k != "" {
		q = q.Where("jsonb_exists(payload->'program_keys',?) OR (payload->>'university_key'=(SELECT u.catalog_key FROM olympguide.university u JOIN olympguide.educational_program p USING(university_id) WHERE p.catalog_key=?) AND jsonb_array_length(COALESCE(payload->'program_keys','[]'))=0)", k, k)
	}
	var rows = []struct {
		ID       string          `json:"id"`
		Revision string          `json:"revision"`
		Payload  json.RawMessage `json:"event"`
	}{}
	if err := q.Select("id,revision,payload").Order("id").Find(&rows).Error; err != nil {
		calendarError(c, err)
		return
	}
	c.JSON(200, rows)
}

type planRow struct {
	PlanID       uint64          `json:"plan_id"`
	EventID      *string         `json:"event_id"`
	Event        json.RawMessage `json:"event"`
	Status       string          `json:"status"`
	Note         string          `json:"note"`
	Revision     string          `json:"revision"`
	DatesChanged bool            `json:"dates_changed"`
	SourceActive bool            `json:"source_active"`
	UpdatedAt    string          `json:"updated_at"`
}

func (h *CalendarHandler) Plan(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	rows := []planRow{}
	err := h.db.WithContext(c.Request.Context()).Raw(`SELECT p.plan_id,p.event_id,COALESCE(e.payload,p.custom_event) AS event,p.status,p.note,
        COALESCE(e.revision,'') AS revision,COALESCE(e.revision<>p.seen_revision,false) AS dates_changed,
        COALESCE(e.active,true) AS source_active,p.updated_at::text FROM olympguide.calendar_plan p
        LEFT JOIN olympguide.calendar_event e ON e.id=p.event_id WHERE p.user_id=? ORDER BY p.plan_id`, user).Scan(&rows).Error
	if err != nil {
		calendarError(c, err)
		return
	}
	c.JSON(200, rows)
}
func (h *CalendarHandler) Add(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	var body struct {
		EventID string `json:"event_id"`
	}
	if !personalJSON(c, &body) {
		return
	}
	var id uint64
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var e struct {
			ID       string
			Revision string
		}
		if err := tx.Table("olympguide.calendar_event").Where("id=? AND active=true", body.EventID).Take(&e).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO olympguide.calendar_plan(user_id,event_id,seen_revision) VALUES(?,?,?) ON CONFLICT(user_id,event_id) DO NOTHING`, user, e.ID, e.Revision).Error; err != nil {
			return err
		}
		return tx.Table("olympguide.calendar_plan").Where("user_id=? AND event_id=?", user, e.ID).Select("plan_id").Scan(&id).Error
	})
	if err != nil {
		calendarError(c, err)
		return
	}
	c.JSON(200, gin.H{"plan_id": id})
}
func (h *CalendarHandler) Custom(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	var body struct {
		ClientKey string                `json:"client_key"`
		Event     service.CalendarEvent `json:"event"`
	}
	if !personalJSON(c, &body) {
		return
	}
	if _, err := uuid.Parse(body.ClientKey); err != nil {
		c.JSON(400, gin.H{"message": "Нужен уникальный ключ события"})
		return
	}
	if err := service.ValidateCalendarEvent(body.Event, true); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	// Custom events cannot impersonate verified source records or inject links.
	body.Event.ID = ""
	body.Event.CheckedOn = ""
	body.Event.UniversityKey = ""
	body.Event.ProgramKeys = nil
	body.Event.OlympiadKeys = nil
	body.Event.WebOlympiadIDs = nil
	payload, _ := json.Marshal(body.Event)
	var id uint64
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO olympguide.calendar_plan(user_id,client_key,custom_event) VALUES(?,?,?::jsonb) ON CONFLICT(user_id,client_key) DO NOTHING`, user, body.ClientKey, string(payload)).Error; err != nil {
			return err
		}
		if err := tx.Table("olympguide.calendar_plan").Where("user_id=? AND client_key=? AND custom_event=?::jsonb", user, body.ClientKey, string(payload)).Select("plan_id").Scan(&id).Error; err != nil {
			return err
		}
		if id == 0 {
			return errCustomKey
		}
		return nil
	})
	if err != nil {
		calendarError(c, err)
		return
	}
	c.JSON(200, gin.H{"plan_id": id})
}
func (h *CalendarHandler) Update(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	id, ok := planID(c)
	if !ok {
		return
	}
	var body struct {
		Status      string                 `json:"status"`
		Note        string                 `json:"note"`
		Acknowledge bool                   `json:"acknowledge"`
		Event       *service.CalendarEvent `json:"event,omitempty"`
	}
	if !personalJSON(c, &body) {
		return
	}
	if !service.CalendarStatus(body.Status) || len([]rune(body.Note)) > 4000 {
		c.JSON(400, gin.H{"message": "Некорректный статус или слишком длинная заметка"})
		return
	}
	if body.Event != nil {
		if err := service.ValidateCalendarEvent(*body.Event, true); err != nil {
			c.JSON(400, gin.H{"message": err.Error()})
			return
		}
	}
	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row struct{ EventID *string }
		if err := tx.Raw("SELECT event_id FROM olympguide.calendar_plan WHERE user_id=? AND plan_id=? FOR UPDATE", user, id).Scan(&row).Error; err != nil {
			return err
		}
		updates := map[string]any{"status": body.Status, "note": strings.TrimSpace(body.Note), "updated_at": gorm.Expr("now()")}
		if body.Event != nil {
			if row.EventID != nil {
				return errSourceEvent
			}
			body.Event.ID = ""
			body.Event.CheckedOn = ""
			body.Event.UniversityKey = ""
			body.Event.ProgramKeys = nil
			body.Event.OlympiadKeys = nil
			body.Event.WebOlympiadIDs = nil
			payload, _ := json.Marshal(body.Event)
			updates["custom_event"] = gorm.Expr("?::jsonb", string(payload))
		}
		if body.Acknowledge {
			updates["seen_revision"] = gorm.Expr("(SELECT revision FROM olympguide.calendar_event WHERE id=calendar_plan.event_id)")
		}
		r := tx.Table("olympguide.calendar_plan").Where("user_id=? AND plan_id=?", user, id).Updates(updates)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		calendarError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "Сохранено"})
}
func (h *CalendarHandler) Delete(c *gin.Context) {
	user, ok := personalUser(c)
	if !ok {
		return
	}
	id, ok := planID(c)
	if !ok {
		return
	}
	r := h.db.WithContext(c.Request.Context()).Exec("DELETE FROM olympguide.calendar_plan WHERE user_id=? AND plan_id=?", user, id)
	if r.Error != nil {
		calendarError(c, r.Error)
		return
	}
	if r.RowsAffected == 0 {
		calendarError(c, gorm.ErrRecordNotFound)
		return
	}
	c.JSON(200, gin.H{"message": "Удалено"})
}
