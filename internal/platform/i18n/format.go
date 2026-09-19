package i18n

import (
	"strings"
	"time"

	"golang.org/x/text/currency"
)

// Money formats an amount held the way money is always held: in minor units,
// with its currency (docs/requirements.md, section 6).
//
// Minor units are what the database stores, because a price is a count of
// cents and never a float — 0.1 + 0.2 is not 0.3 in binary, and a marketplace
// that rounds a commission the wrong way owes somebody money. The division
// happens here, once, at the edge where a person reads it.
func (c *Catalogue) Money(tag string, minorUnits int64, code string) (string, error) {
	unit, err := currency.ParseISO(code)
	if err != nil {
		return "", err
	}

	scale, _ := currency.Cash.Rounding(unit)
	divisor := 1.0
	for range scale {
		divisor *= 10
	}

	return c.Printer(tag).Sprint(currency.Symbol(unit.Amount(float64(minorUnits) / divisor))), nil
}

// Date formats an instant for a language, from a layout the catalogue holds.
//
// The layout is data rather than code because a date's shape is part of a
// locale, not of a program: `Jan 2, 2006` in en-US and `02/01/2006` in pt-BR,
// and a third language changes nothing but its own file.
//
// Go writes month and weekday names in English whatever the layout, so the
// names are translated afterwards from the same catalogue. The instant is
// converted out of UTC by the caller, which knows the visitor's zone; what is
// stored is always UTC (docs/requirements.md, section 6).
func (c *Catalogue) Date(tag string, when time.Time, key string) string {
	layout := c.Printer(tag).Sprintf(key)
	return c.translateNames(tag, when.Format(layout), when)
}

// translateNames replaces the English month and weekday names Go wrote with
// the ones this language uses.
//
// Whole names only. Go abbreviates to the first three letters of the English
// name, and a language whose month is not three letters longer cannot be
// abbreviated by cutting the English one — a catalogue that wants abbreviated
// names says so in its layout and gets them from its own keys.
func (c *Catalogue) translateNames(tag, written string, when time.Time) string {
	month := when.Month().String()
	if strings.Contains(written, month) {
		written = strings.ReplaceAll(written, month, c.Printer(tag).Sprintf("month."+strings.ToLower(month)))
	}

	weekday := when.Weekday().String()
	if strings.Contains(written, weekday) {
		written = strings.ReplaceAll(written, weekday, c.Printer(tag).Sprintf("weekday."+strings.ToLower(weekday)))
	}
	return written
}
