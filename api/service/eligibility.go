package service

import (
	"fmt"
	"strings"
)

type ExamResult struct {
	Subject string `json:"subject"`
	Score   int    `json:"score"`
	Year    int    `json:"year"`
}
type ApplicantProfile struct {
	AdmissionYear *int         `json:"admission_year"`
	Exams         []ExamResult `json:"exams"`
}
type DiplomaFacts struct {
	AwardYear    *int
	OlympiadYear *string
	Profile      *string
	Class        uint
	Result       *string
}
type Requirement struct {
	All             []Requirement `json:"all,omitempty"`
	Any             []Requirement `json:"any,omitempty"`
	Field           string        `json:"field,omitempty"`
	Values          []string      `json:"values,omitempty"`
	Min             *int          `json:"min,omitempty"`
	Max             *int          `json:"max,omitempty"`
	Subject         string        `json:"subject,omitempty"`
	Label           string        `json:"label,omitempty"`
	ManualOnFailure bool          `json:"manual_on_failure,omitempty"`
}
type EligibilityRule struct {
	ID            string      `json:"id"`
	AdmissionYear int         `json:"admission_year"`
	Benefit       string      `json:"benefit"`
	Complete      bool        `json:"complete"`
	ManualReason  string      `json:"manual_reason"`
	Scope         string      `json:"scope"`
	ProgramKeys   []string    `json:"program_keys"`
	Requirement   Requirement `json:"requirement"`
}
type ConditionCheck struct {
	Condition string `json:"condition"`
	State     string `json:"state"`
	Detail    string `json:"detail"`
}
type Assessment struct {
	Status  string           `json:"status"`
	Benefit string           `json:"benefit"`
	Reason  string           `json:"reason"`
	Checks  []ConditionCheck `json:"checks"`
}

const (
	Eligible             = "eligible"
	ConfirmationRequired = "confirmation_required"
	Ineligible           = "ineligible"
	ManualReview         = "manual_review"
)

func normalizeFact(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(s, "ё", "е"))), " ")
}
func listed(value string, values []string) bool {
	for _, v := range values {
		if normalizeFact(value) == normalizeFact(v) {
			return true
		}
	}
	return false
}

func factLabel(field, value string) string {
	if field == "result" {
		switch value {
		case "winner":
			return "победитель"
		case "prize_winner":
			return "призёр"
		case "participant":
			return "участник"
		}
	}
	return value
}

func EvaluateEligibility(rule EligibilityRule, d DiplomaFacts, p ApplicantProfile, programKey string, sourceUnchanged bool) Assessment {
	result := Assessment{Status: ManualReview, Benefit: rule.Benefit, Checks: []ConditionCheck{}}
	if p.AdmissionYear == nil || *p.AdmissionYear != rule.AdmissionYear {
		result.Reason = "Для выбранного года поступления нет проверенного правила"
		return result
	}
	if !sourceUnchanged && rule.Complete {
		result.Reason = "Источник изменился после разбора: нужна повторная проверка"
		return result
	}
	if !rule.Complete {
		result.Reason = rule.ManualReason
		if result.Reason == "" {
			result.Reason = "Условие разобрано не полностью"
		}
		return result
	}
	if !listed(programKey, rule.ProgramKeys) {
		result.Reason = "Применимость правила к этой программе не подтверждена"
		return result
	}
	if !validRequirement(rule.Requirement) {
		result.Reason = "Неизвестная или неполная структура условия: нужна ручная проверка"
		return result
	}
	state, checks := evaluateRequirement(rule.Requirement, d, p)
	result.Checks = append([]ConditionCheck{{Condition: "Область действия", State: "pass", Detail: rule.Scope}}, checks...)
	switch state {
	case "pass":
		result.Status = Eligible
		result.Reason = "Проверенные условия льготы выполнены по указанным вами данным"
	case "pending":
		result.Status = ConfirmationRequired
		result.Reason = "Параметры диплома подходят; добавьте подтверждающий результат ЕГЭ"
	case "fail":
		result.Status = Ineligible
		result.Reason = "Условия этого правила не выполнены"
	default:
		result.Status = ManualReview
		result.Reason = "Недостаточно данных или требуется ручная проверка"
	}
	return result
}

func evaluateRequirement(r Requirement, d DiplomaFacts, p ApplicantProfile) (string, []ConditionCheck) {
	if len(r.All) > 0 || len(r.Any) > 0 {
		children := r.All
		isAny := len(r.Any) > 0
		if isAny {
			children = r.Any
			if len(children) == 1 {
				return evaluateRequirement(children[0], d, p)
			}
		}
		states := map[string]bool{}
		traces := map[string][]ConditionCheck{}
		checks := []ConditionCheck{}
		for _, c := range children {
			s, trace := evaluateRequirement(c, d, p)
			states[s] = true
			if _, exists := traces[s]; !exists {
				traces[s] = trace
			}
			checks = append(checks, trace...)
		}
		if isAny {
			for _, s := range []string{"pass", "pending", "unknown", "fail"} {
				if states[s] {
					return s, append([]ConditionCheck{{Condition: "Альтернативные условия", State: s, Detail: "Достаточно одного варианта. Показан наиболее подходящий; остальные приведены в источнике"}}, traces[s]...)
				}
			}
		}
		for _, s := range []string{"fail", "unknown", "pending", "pass"} {
			if states[s] {
				return s, checks
			}
		}
	}
	state, detail := "unknown", "Не указано"
	known := false
	value := ""
	number := 0
	switch r.Field {
	case "award_year":
		if d.AwardYear != nil {
			known = true
			number = *d.AwardYear
			value = fmt.Sprint(number)
		}
	case "olympiad_year":
		if d.OlympiadYear != nil && *d.OlympiadYear != "" {
			known = true
			value = *d.OlympiadYear
		}
	case "profile":
		if d.Profile != nil && *d.Profile != "" {
			known = true
			value = *d.Profile
		}
	case "class":
		if d.Class != 0 {
			known = true
			number = int(d.Class)
			value = fmt.Sprint(number)
		}
	case "result":
		if d.Result != nil && *d.Result != "" {
			known = true
			value = *d.Result
		}
	case "exam":
		best := -1
		hasSubject := false
		for _, e := range p.Exams {
			if normalizeFact(e.Subject) == normalizeFact(r.Subject) {
				hasSubject = true
				if p.AdmissionYear != nil && e.Year <= *p.AdmissionYear && e.Year >= *p.AdmissionYear-4 && e.Score > best {
					best = e.Score
				}
			}
		}
		if best < 0 {
			state = "pending"
			detail = "Добавьте действующий результат ЕГЭ по предмету «" + r.Subject + "»"
			if hasSubject {
				detail = "Указанный год ЕГЭ по предмету «" + r.Subject + "» не подтверждает приём этого года"
			}
		} else {
			known = true
			number = best
			value = fmt.Sprint(best)
		}
	default:
		detail = "Неизвестный тип условия: нужна ручная проверка"
	}
	if known {
		pass := true
		if len(r.Values) > 0 {
			pass = listed(value, r.Values)
		}
		if r.Min != nil && number < *r.Min {
			pass = false
		}
		if r.Max != nil && number > *r.Max {
			pass = false
		}
		state = "pass"
		if !pass {
			state = "fail"
			if r.ManualOnFailure {
				state = "unknown"
			}
		}
		detail = "Указано: " + factLabel(r.Field, value)
		if len(r.Values) > 0 {
			labels := make([]string, len(r.Values))
			for i, v := range r.Values {
				labels[i] = factLabel(r.Field, v)
			}
			detail += "; требуется: " + strings.Join(labels, ", ")
		}
		if r.Min != nil {
			detail += fmt.Sprintf("; минимум %d", *r.Min)
		}
		if r.Max != nil {
			detail += fmt.Sprintf("; максимум %d", *r.Max)
		}
		if state == "unknown" {
			detail += ". Возможное исключение или другой год требует проверки"
		}
	}
	return state, []ConditionCheck{{Condition: r.Label, State: state, Detail: detail}}
}

func validRequirement(r Requirement) bool {
	if len(r.All) > 0 || len(r.Any) > 0 {
		if (len(r.All) > 0 && len(r.Any) > 0) || r.Field != "" {
			return false
		}
		for _, child := range append(append([]Requirement{}, r.All...), r.Any...) {
			if !validRequirement(child) {
				return false
			}
		}
		return true
	}
	if r.Label == "" || (r.Min == nil && r.Max == nil && len(r.Values) == 0) {
		return false
	}
	if r.Min != nil && r.Max != nil && *r.Min > *r.Max {
		return false
	}
	switch r.Field {
	case "award_year", "class":
		return true
	case "profile", "olympiad_year", "result":
		return len(r.Values) > 0 && r.Min == nil && r.Max == nil
	case "exam":
		return r.Subject != "" && r.Min != nil
	default:
		return false
	}
}

func ValidateApplicantProfile(p ApplicantProfile) error {
	if p.AdmissionYear != nil && (*p.AdmissionYear < 2000 || *p.AdmissionYear > 2100) {
		return fmt.Errorf("Некорректный год поступления")
	}
	if len(p.Exams) > 40 {
		return fmt.Errorf("Слишком много результатов ЕГЭ")
	}
	seen := map[string]bool{}
	for _, e := range p.Exams {
		if !listed(e.Subject, []string{"математика", "информатика", "физика", "химия", "биология", "русский язык", "литература", "история", "обществознание", "география", "иностранный язык"}) || e.Score < 0 || e.Score > 100 || e.Year < 2000 || e.Year > 2100 {
			return fmt.Errorf("Проверьте предмет, баллы и год ЕГЭ")
		}
		k := fmt.Sprintf("%s:%d", normalizeFact(e.Subject), e.Year)
		if seen[k] {
			return fmt.Errorf("Один результат на предмет и год")
		}
		seen[k] = true
	}
	return nil
}
