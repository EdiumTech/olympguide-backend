package service

import (
	"api/dto"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func ValidateDiplomaDetails(d dto.DiplomaDetails, class, level uint) error {
	if class < 1 || class > 11 || level < 1 || level > 3 {
		return fmt.Errorf("Проверьте класс и степень диплома")
	}
	if d.AwardYear != nil && (*d.AwardYear < 2000 || *d.AwardYear > 2100) {
		return fmt.Errorf("Проверьте год получения")
	}
	if d.OlympiadYear != nil {
		year := *d.OlympiadYear
		if !regexp.MustCompile(`^20\d\d/20\d\d$`).MatchString(year) {
			return fmt.Errorf("Учебный год: ГГГГ/ГГГГ")
		}
		first, _ := strconv.Atoi(year[:4])
		last, _ := strconv.Atoi(year[5:])
		if last != first+1 {
			return fmt.Errorf("Учебный год должен содержать два соседних года")
		}
		if d.AwardYear != nil && (*d.AwardYear < first || *d.AwardYear > last) {
			return fmt.Errorf("Год получения не согласован с учебным годом олимпиады")
		}
	}
	if d.Profile != nil && (strings.TrimSpace(*d.Profile) == "" || len([]rune(*d.Profile)) > 300) {
		return fmt.Errorf("Проверьте профиль олимпиады")
	}
	if d.Result != nil {
		if !listed(*d.Result, []string{"winner", "prize_winner", "participant"}) {
			return fmt.Errorf("Неизвестный результат диплома")
		}
		if (*d.Result == "winner" && level != 1) || (*d.Result == "prize_winner" && level == 1) {
			return fmt.Errorf("Результат не согласован со степенью диплома")
		}
	}
	return nil
}
