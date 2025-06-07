// Transliteration of JSONata from Javascript to Go
// Performed June 2025 by Ray Ozzie using Claude Code
// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

/*
JSONata Date/Time Functions Implementation
=========================================

Portions Copyright IBM Corp. 2018 All Rights Reserved
Project name: JSONata
This project is licensed under the MIT License, see LICENSE

Overview:
--------
This module implements JSONata's date and time handling capabilities, providing
functions for parsing, formatting, and manipulating temporal data. The implementation
follows the XPath Functions specification for date/time formatting while adapting
to Go's time handling model.

Core Functions:
--------------

1. **$now()** - Current timestamp with optional formatting
   Returns the current date/time, optionally formatted according to a picture string

2. **$millis()** - Current time as milliseconds since Unix epoch
   Provides numeric timestamp for calculations and comparisons

3. **$fromMillis()** - Convert milliseconds to formatted date string
   Transforms numeric timestamps into human-readable date/time strings

4. **$toMillis()** - Parse date string to milliseconds
   Converts formatted date strings back to numeric timestamps

5. **$formatInteger()** - Format numbers as words or with custom formatting
   Supports cardinal/ordinal number conversion and custom formatting pictures

Date/Time Formatting:
-------------------
The formatting system uses "picture strings" that define output format:
- '[Y0001]' - 4-digit year with zero padding
- '[M01]' - 2-digit month with zero padding
- '[D01]' - 2-digit day with zero padding
- '[H01]' - 2-digit hour (24-hour format)
- '[m01]' - 2-digit minute
- '[s01]' - 2-digit second

Example formats:
- '[Y0001]-[M01]-[D01]' → '2023-12-25'
- '[FNn] [D1] [MNn] [Y0001]' → 'Monday 25 December 2023'
- '[Y] [MNn,*-3] [D1o]' → '2023 December 25th'

Number Formatting:
-----------------
Integer formatting supports multiple presentation modes:
- Cardinal numbers: 'one', 'two', 'three'
- Ordinal numbers: 'first', 'second', 'third'
- Roman numerals: 'i', 'ii', 'iii' (lower) or 'I', 'II', 'III' (upper)
- Custom formatting with padding and grouping

Implementation Notes:
-------------------
- Uses Go's time.Time for internal representation
- Handles timezone conversions and DST correctly
- Supports leap years and calendar edge cases
- Performance optimized for common formatting patterns
- Error handling follows JSONata's graceful degradation principles
*/

package jsonata

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

/*
Global DateTime Module Instance
==============================

The dateTime module provides all temporal functions for JSONata expressions.
This singleton instance is initialized once and shared across all expression
evaluations, providing consistent date/time behavior and efficient resource usage.

The module implements the XPath Functions specification for date/time formatting,
adapted for JSONata's needs and Go's time model.
*/
var dateTime = initDateTime()

/*
initDateTime - Initialize Date/Time Function Module
==================================================

Creates and configures the complete date/time function library, including
formatters, parsers, and utility functions for temporal data manipulation.

The initialization process establishes:
1. Number-to-words conversion tables for formatting
2. Date/time formatting picture string processors
3. Timezone and calendar handling functions
4. Integer formatting with multiple presentation modes

This comprehensive setup enables JSONata's rich temporal data processing
capabilities while maintaining performance through pre-computed lookup tables.
*/
func initDateTime() *DateTimeModule {
	// Dummy reference for unused import
	_ = sort.Strings

	// Capture utility function for string processing
	stringToArray := stringToArray

	// Number-to-words conversion tables for cardinal numbers (0-19)
	// Used for formatting integers as English words: "Zero", "One", "Two", etc.
	few := []string{"Zero", "One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten",
		"Eleven", "Twelve", "Thirteen", "Fourteen", "Fifteen", "Sixteen", "Seventeen", "Eighteen", "Nineteen"}

	// Ordinal number words for position formatting: "First", "Second", "Third", etc.
	// Used when formatting numbers with ordinal presentation (1st, 2nd, 3rd)
	ordinals := []string{"Zeroth", "First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth", "Ninth", "Tenth",
		"Eleventh", "Twelfth", "Thirteenth", "Fourteenth", "Fifteenth", "Sixteenth", "Seventeenth", "Eighteenth", "Nineteenth"}

	// Decade names for numbers 20-90, plus "Hundred" for larger number formatting
	decades := []string{"Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety", "Hundred"}

	// Magnitude names for large number formatting (thousands, millions, etc.)
	magnitudes := []string{"Thousand", "Million", "Billion", "Trillion"}

	/**
	 * converts a number into english words
	 * @param {int} value - the value to format
	 * @param {bool} ordinal - ordinal or cardinal form
	 * @returns {string} - representation in words
	 */
	numberToWords := func(value int, ordinal bool) string {
		var lookup func(num int, prev bool, ord bool) string
		lookup = func(num int, prev bool, ord bool) string {
			words := ""
			if num <= 19 {
				prefix := ""
				if prev {
					prefix = " and "
				}
				if ord {
					words = prefix + ordinals[num]
				} else {
					words = prefix + few[num]
				}
			} else if num < 100 {
				tens := num / 10
				remainder := num % 10
				prefix := ""
				if prev {
					prefix = " and "
				}
				words = prefix + decades[tens-2]
				if remainder > 0 {
					words += "-" + lookup(remainder, false, ord)
				} else if ord {
					words = words[:len(words)-1] + "ieth"
				}
			} else if num < 1000 {
				hundreds := num / 100
				remainder := num % 100
				prefix := ""
				if prev {
					prefix = ", "
				}
				words = prefix + few[hundreds] + " Hundred"
				if remainder > 0 {
					words += lookup(remainder, true, ord)
				} else if ord {
					words += "th"
				}
			} else {
				mag := int(math.Floor(math.Log10(float64(num)) / 3))
				if mag > len(magnitudes) {
					mag = len(magnitudes) // the largest word
				}
				factor := int(math.Pow(10, float64(mag*3)))
				mant := num / factor
				remainder := num - mant*factor
				prefix := ""
				if prev {
					prefix = ", "
				}
				words = prefix + lookup(mant, false, false) + " " + magnitudes[mag-1]
				if remainder > 0 {
					words += lookup(remainder, true, ord)
				} else if ord {
					words += "th"
				}
			}
			return words
		}

		words := lookup(value, false, ordinal)
		return words
	}

	wordValues := make(map[string]int)
	for index, word := range few {
		wordValues[strings.ToLower(word)] = index
	}
	for index, word := range ordinals {
		wordValues[strings.ToLower(word)] = index
	}
	for index, word := range decades {
		lword := strings.ToLower(word)
		wordValues[lword] = (index + 2) * 10
		wordValues[lword[:len(word)-1]+"ieth"] = wordValues[lword]
	}
	wordValues["hundredth"] = 100
	for index, word := range magnitudes {
		lword := strings.ToLower(word)
		val := int(math.Pow(10, float64((index+1)*3)))
		wordValues[lword] = val
		wordValues[lword+"th"] = val
	}

	/**
	 * Converts a number in english words to numeric value
	 * @param {string} text - the number in words
	 * @returns {int} - the numeric value
	 */
	wordsToNumber := func(text string) int {
		re := regexp.MustCompile(`,\s|\sand\s|[\s\\-]`)
		parts := re.Split(text, -1)
		values := []int{}
		for _, part := range parts {
			if val, ok := wordValues[part]; ok {
				values = append(values, val)
			}
		}
		segs := []int{0}
		for _, value := range values {
			if value < 100 {
				top := segs[len(segs)-1]
				segs = segs[:len(segs)-1]
				if top >= 1000 {
					segs = append(segs, top)
					top = 0
				}
				segs = append(segs, top+value)
			} else {
				top := segs[len(segs)-1]
				segs[len(segs)-1] = top * value
			}
		}
		result := 0
		for _, seg := range segs {
			result += seg
		}
		return result
	}

	romanNumerals := []struct {
		value   int
		numeral string
	}{
		{1000, "m"},
		{900, "cm"},
		{500, "d"},
		{400, "cd"},
		{100, "c"},
		{90, "xc"},
		{50, "l"},
		{40, "xl"},
		{10, "x"},
		{9, "ix"},
		{5, "v"},
		{4, "iv"},
		{1, "i"},
	}

	romanValues := map[rune]int{'M': 1000, 'D': 500, 'C': 100, 'L': 50, 'X': 10, 'V': 5, 'I': 1}

	/**
	 * converts a number to roman numerals
	 * @param {int} value - the number
	 * @returns {string} - the number in roman numerals
	 */
	var decimalToRoman func(int) string
	decimalToRoman = func(value int) string {
		for _, numeral := range romanNumerals {
			if value >= numeral.value {
				return numeral.numeral + decimalToRoman(value-numeral.value)
			}
		}
		return ""
	}

	/**
	 * converts roman numerals to a number
	 * @param {string} roman - roman number
	 * @returns {int} - the numeric value
	 */
	romanToDecimal := func(roman string) int {
		decimal := 0
		max := 1
		runes := []rune(roman)
		for i := len(runes) - 1; i >= 0; i-- {
			digit := runes[i]
			value := romanValues[unicode.ToUpper(digit)]
			if value < max {
				decimal -= value
			} else {
				max = value
				decimal += value
			}
		}
		return decimal
	}

	/**
	 * converts a number to spreadsheet style letters
	 * @param {int} value - the number
	 * @param {rune} aChar - the character representing the start of the sequence, e.g. 'A'
	 * @returns {string} - the letters
	 */
	decimalToLetters := func(value int, aChar rune) string {
		letters := []string{}
		aCode := int(aChar)
		for value > 0 {
			letters = append([]string{string(rune((value-1)%26 + aCode))}, letters...)
			value = (value - 1) / 26
		}
		return strings.Join(letters, "")
	}

	/**
	 * converts spreadsheet style letters to a number
	 * @param {string} letters - the letters
	 * @param {rune} aChar - the character representing the start of the sequence, e.g. 'A'
	 * @returns {int} - the numeric value
	 */
	lettersToDecimal := func(letters string, aChar rune) int {
		aCode := int(aChar)
		decimal := 0
		runes := []rune(letters)
		for i := 0; i < len(runes); i++ {
			decimal += (int(runes[len(runes)-i-1]) - aCode + 1) * int(math.Pow(26, float64(i)))
		}
		return decimal
	}

	// Forward declarations
	var analyseIntegerPicture func(string) (*IntegerFormat, error)
	var _formatInteger func(int, *IntegerFormat) (string, error)
	var generateRegex func(interface{}) (*Matcher, error)

	/**
	 * Formats an integer as specified by the XPath fn:format-integer function
	 * See https://www.w3.org/TR/xpath-functions-31/#func-format-integer
	 * @param {interface{}} value - the number to be formatted
	 * @param {string} picture - the picture string that specifies the format
	 * @returns {string} - the formatted number
	 * JavaScript: datetime.js line 214
	 */
	formatInteger := func(value interface{}, picture string) (string, error) {
		if value == nil {
			return "", nil
		}

		intValue := 0
		switch v := value.(type) {
		case int:
			intValue = v
		case float64:
			// Check for overflow when converting to int
			if v >= float64(math.MaxInt64) {
				// For word formatting, return an error for numbers beyond int64 range
				if picture == "w" || picture == "W" || picture == "w;o" || picture == "W;o" {
					return "", &JSONataError{
						Code:  "D3100", // Using a number formatting error code
						Value: fmt.Sprintf("Number too large for word formatting: %.0f", v),
					}
				}
				// For other formats, cap at MaxInt64
				intValue = math.MaxInt64
			} else {
				intValue = int(math.Floor(v))
			}
		case float32:
			intValue = int(math.Floor(float64(v)))
		default:
			return "", fmt.Errorf("invalid integer value")
		}

		format, err := analyseIntegerPicture(picture)
		if err != nil {
			return "", err
		}
		return _formatInteger(intValue, format)
	}

	formats := struct {
		DECIMAL  string
		LETTERS  string
		ROMAN    string
		WORDS    string
		SEQUENCE string
	}{
		DECIMAL:  "decimal",
		LETTERS:  "letters",
		ROMAN:    "roman",
		WORDS:    "words",
		SEQUENCE: "sequence",
	}

	tcase := struct {
		UPPER string
		LOWER string
		TITLE string
	}{
		UPPER: "upper",
		LOWER: "lower",
		TITLE: "title",
	}

	/**
	 * formats an integer using a preprocessed representation of the picture string
	 * @param {int} value - the number to be formatted
	 * @param {*IntegerFormat} format - the preprocessed representation of the picture string
	 * @returns {string} - the formatted number
	 * @private
	 */
	_formatInteger = func(value int, format *IntegerFormat) (string, error) {
		formattedInteger := ""
		negative := value < 0
		value = int(math.Abs(float64(value)))
		switch format.primary {
		case formats.LETTERS:
			if format.case_ == tcase.UPPER {
				formattedInteger = decimalToLetters(value, 'A')
			} else {
				formattedInteger = decimalToLetters(value, 'a')
			}
		case formats.ROMAN:
			formattedInteger = decimalToRoman(value)
			if format.case_ == tcase.UPPER {
				formattedInteger = strings.ToUpper(formattedInteger)
			}
		case formats.WORDS:
			formattedInteger = numberToWords(value, format.ordinal)
			if format.case_ == tcase.UPPER {
				formattedInteger = strings.ToUpper(formattedInteger)
			} else if format.case_ == tcase.LOWER {
				formattedInteger = strings.ToLower(formattedInteger)
			}
		case formats.DECIMAL:
			formattedInteger = strconv.Itoa(value)
			// TODO use functionPad (JavaScript: datetime.js line 270)
			padLength := format.mandatoryDigits - len(formattedInteger)
			if padLength > 0 {
				padding := strings.Repeat("0", padLength)
				formattedInteger = padding + formattedInteger
			}
			if format.zeroCode != 0x30 {
				chars := stringToArray(formattedInteger)
				newChars := []string{}
				for _, char := range chars {
					r := []rune(char)[0]
					newR := r + rune(format.zeroCode) - 0x30
					newChars = append(newChars, string(newR))
				}
				formattedInteger = strings.Join(newChars, "")
			}
			// insert the grouping-separator-signs, if any
			if format.regular {
				n := (len(formattedInteger) - 1) / format.groupingSeparators.position
				for ii := n; ii > 0; ii-- {
					pos := len(formattedInteger) - ii*format.groupingSeparators.position
					formattedInteger = formattedInteger[:pos] + format.groupingSeparators.character + formattedInteger[pos:]
				}
			} else {
				// Reverse the separators slice
				reversedSeps := make([]GroupingSeparator, len(format.groupingSeparatorsList))
				for i := range format.groupingSeparatorsList {
					reversedSeps[i] = format.groupingSeparatorsList[len(format.groupingSeparatorsList)-1-i]
				}
				for _, separator := range reversedSeps {
					pos := len(formattedInteger) - separator.position
					if pos >= 0 && pos <= len(formattedInteger) {
						formattedInteger = formattedInteger[:pos] + separator.character + formattedInteger[pos:]
					}
				}
			}

			if format.ordinal {
				suffix123 := map[string]string{"1": "st", "2": "nd", "3": "rd"}
				lastDigit := string(formattedInteger[len(formattedInteger)-1])
				suffix := suffix123[lastDigit]
				if suffix == "" || (len(formattedInteger) > 1 && formattedInteger[len(formattedInteger)-2] == '1') {
					suffix = "th"
				}
				formattedInteger = formattedInteger + suffix
			}
		case formats.SEQUENCE:
			return "", &JSONataError{
				Code:  "D3130",
				Value: format.token,
			}
		}
		if negative {
			formattedInteger = "-" + formattedInteger
		}

		return formattedInteger, nil
	}

	//TODO what about decimal groups in the unicode supplementary planes (surrogate pairs) ??? (JavaScript: datetime.js line 318)
	decimalGroups := []int{0x30, 0x0660, 0x06F0, 0x07C0, 0x0966, 0x09E6, 0x0A66, 0x0AE6, 0x0B66, 0x0BE6, 0x0C66, 0x0CE6, 0x0D66, 0x0DE6, 0x0E50, 0x0ED0, 0x0F20, 0x1040, 0x1090, 0x17E0, 0x1810, 0x1946, 0x19D0, 0x1A80, 0x1A90, 0x1B50, 0x1BB0, 0x1C40, 0x1C50, 0xA620, 0xA8D0, 0xA900, 0xA9D0, 0xA9F0, 0xAA50, 0xABF0, 0xFF10}

	/**
	 * preprocesses the picture string
	 * @param {string} picture - picture string
	 * @returns {*IntegerFormat} - analysed picture
	 * JavaScript: datetime.js line 326
	 */
	analyseIntegerPicture = func(picture string) (*IntegerFormat, error) {
		format := &IntegerFormat{
			type_:   "integer",
			primary: formats.DECIMAL,
			case_:   tcase.LOWER,
			ordinal: false,
		}

		var primaryFormat, formatModifier string
		semicolon := strings.LastIndex(picture, ";")
		if semicolon == -1 {
			primaryFormat = picture
		} else {
			primaryFormat = picture[:semicolon]
			formatModifier = picture[semicolon+1:]
			if len(formatModifier) > 0 && formatModifier[0] == 'o' {
				format.ordinal = true
			}
		}

		/* eslnt-disable-next no-fallthrough */
		switch primaryFormat {
		case "A":
			format.case_ = tcase.UPPER
			fallthrough
		/* eslnt-disable-next-line no-fallthrough */
		case "a":
			format.primary = formats.LETTERS
		case "I":
			format.case_ = tcase.UPPER
			fallthrough
		/* eslnt-disable-next-line no-fallthrough */
		case "i":
			format.primary = formats.ROMAN
		case "W":
			format.case_ = tcase.UPPER
			format.primary = formats.WORDS
		case "Ww":
			format.case_ = tcase.TITLE
			format.primary = formats.WORDS
		case "w":
			format.primary = formats.WORDS
		default:
			// this is a decimal-digit-pattern if it contains a decimal digit (from any unicode decimal digit group)
			var zeroCode *int
			mandatoryDigits := 0
			optionalDigits := 0
			groupingSeparators := []GroupingSeparator{}
			separatorPosition := 0
			formatCodepoints := []rune(primaryFormat)
			// reverse the array to determine positions of grouping-separator-signs
			for i, j := 0, len(formatCodepoints)-1; i < j; i, j = i+1, j-1 {
				formatCodepoints[i], formatCodepoints[j] = formatCodepoints[j], formatCodepoints[i]
			}
			for _, codePoint := range formatCodepoints {
				// step though each char in the picture to determine the digit group
				digit := false
				for _, group := range decimalGroups {
					if int(codePoint) >= group && int(codePoint) <= group+9 {
						// codepoint is part of this decimal group
						digit = true
						mandatoryDigits++
						separatorPosition++
						if zeroCode == nil {
							g := group
							zeroCode = &g
						} else if group != *zeroCode {
							// error! different decimal groups in the same pattern
							return nil, &JSONataError{
								Code: "D3131",
							}
						}
						break
					}
				}
				if !digit {
					if codePoint == 0x23 { // # - optional-digit-sign
						separatorPosition++
						optionalDigits++
					} else {
						// neither a decimal-digit-sign ot optional-digit-sign, assume it is a grouping-separator-sign
						groupingSeparators = append(groupingSeparators, GroupingSeparator{
							position:  separatorPosition,
							character: string(codePoint),
						})
					}
				}
			}
			if mandatoryDigits > 0 {
				format.primary = formats.DECIMAL
				// TODO validate decimal-digit-pattern (JavaScript: datetime.js line 415)

				// the decimal digit family (codepoint offset)
				if zeroCode != nil {
					format.zeroCode = *zeroCode
				}
				// the number of mandatory digits
				format.mandatoryDigits = mandatoryDigits
				// the number of optional digits
				format.optionalDigits = optionalDigits
				// grouping separator template
				// are the grouping-separator-signs 'regular'?
				regularRepeat := func(separators []GroupingSeparator) int {
					// are the grouping positions regular? i.e. same interval between each of them
					// is there at least one separator?
					if len(separators) == 0 {
						return 0
					}
					// are all the characters the same?
					sepChar := separators[0].character
					for ii := 1; ii < len(separators); ii++ {
						if separators[ii].character != sepChar {
							return 0
						}
					}
					// are they equally spaced?
					indexes := []int{}
					for _, sep := range separators {
						indexes = append(indexes, sep.position)
					}
					gcd := func(a, b int) int {
						for b != 0 {
							a, b = b, a%b
						}
						return a
					}
					// find the greatest common divisor of all the positions
					factor := indexes[0]
					for _, idx := range indexes[1:] {
						factor = gcd(factor, idx)
					}
					// is every position separated by this divisor? If so, it's regular
					for index := 1; index <= len(indexes); index++ {
						found := false
						for _, idx := range indexes {
							if idx == index*factor {
								found = true
								break
							}
						}
						if !found {
							return 0
						}
					}
					return factor
				}

				regular := regularRepeat(groupingSeparators)
				if regular > 0 {
					format.regular = true
					format.groupingSeparators = GroupingSeparator{
						position:  regular,
						character: groupingSeparators[0].character,
					}
				} else {
					format.regular = false
					format.groupingSeparatorsList = groupingSeparators
				}

			} else {
				// this is a 'numbering sequence' which the spec says is implementation-defined
				// this implementation doesn't support any numbering sequences at the moment.
				format.primary = formats.SEQUENCE
				format.token = primaryFormat
			}
		}

		return format, nil
	}

	defaultPresentationModifiers := map[string]string{
		"Y": "1", "M": "1", "D": "1", "d": "1", "F": "n", "W": "1", "w": "1", "X": "1", "x": "1", "H": "1", "h": "1",
		"P": "n", "m": "01", "s": "01", "f": "1", "Z": "01:01", "z": "01:01", "C": "n", "E": "n",
	}

	// §9.8.4.1 the format specifier is an array of string literals and variable markers
	/**
	 * analyse the date-time picture string
	 * @param {string} picture - picture string
	 * @returns {*DateTimeFormat} - the analysed string
	 * JavaScript: datetime.js line 489
	 */
	analyseDateTimePicture := func(picture string) (*DateTimeFormat, error) {
		spec := []DateTimePart{}
		format := &DateTimeFormat{
			type_: "datetime",
			parts: spec,
		}
		addLiteral := func(start, end int) {
			if end > start {
				literal := picture[start:end]
				// replace any doubled ]] with single ]
				// what if there are instances of single ']' ? - the spec doesn't say
				literal = strings.ReplaceAll(literal, "]]", "]")
				spec = append(spec, DateTimePart{type_: "literal", value: literal})
			}
		}

		start := 0
		pos := 0
		for pos < len(picture) {
			if pos < len(picture) && picture[pos] == '[' {
				// check it's not a doubled [[
				if pos+1 < len(picture) && picture[pos+1] == '[' {
					// literal [
					addLiteral(start, pos)
					spec = append(spec, DateTimePart{type_: "literal", value: "["})
					pos += 2
					start = pos
					continue
				}
				// start of variable marker
				// push the string literal (if there is one) onto the array
				addLiteral(start, pos)
				start = pos
				// search forward to closing ]
				pos = strings.Index(picture[start:], "]")
				if pos == -1 {
					// error - no closing bracket
					return nil, &JSONataError{
						Code: "D3135",
					}
				}
				pos += start
				marker := picture[start+1 : pos]
				// whitespace within a variable marker is ignored (i.e. remove it)
				re := regexp.MustCompile(`\s+`)
				marker = re.ReplaceAllString(marker, "")
				def := DateTimePart{
					type_:     "marker",
					component: string(marker[0]), // 1. The component specifier is always present and is always a single letter.
				}
				comma := strings.LastIndex(marker, ",") // 2. The width modifier may be recognized by the presence of a comma
				var presMod string                      // the presentation modifiers
				if comma != -1 {
					// §9.8.4.2 The Width Modifier
					widthMod := marker[comma+1:]
					dash := strings.Index(widthMod, "-")
					var min, max string
					parseWidth := func(wm string) (*int, error) {
						if wm == "" || wm == "*" {
							return nil, nil
						} else {
							// TODO validate wm is an unsigned int (JavaScript: datetime.js line 548)
							val, err := strconv.Atoi(wm)
							if err != nil {
								return nil, err
							}
							return &val, nil
						}
					}
					if dash == -1 {
						min = widthMod
					} else {
						min = widthMod[:dash]
						max = widthMod[dash+1:]
					}
					minWidth, err := parseWidth(min)
					if err != nil {
						return nil, err
					}
					maxWidth, err := parseWidth(max)
					if err != nil {
						return nil, err
					}
					widthDef := WidthDef{
						min: minWidth,
						max: maxWidth,
					}
					def.width = &widthDef
					presMod = marker[1:comma]
				} else {
					presMod = marker[1:]
				}
				if len(presMod) == 1 {
					def.presentation1 = presMod // first presentation modifier
					//TODO validate the first presentation modifier - it's either N, n, Nn or it passes analyseIntegerPicture (JavaScript: datetime.js line 569)
				} else if len(presMod) > 1 {
					lastChar := presMod[len(presMod)-1]
					if strings.ContainsRune("atco", rune(lastChar)) {
						def.presentation2 = string(lastChar)
						if lastChar == 'o' {
							def.ordinal = true
						}
						// 'c' means 'cardinal' and is the default (i.e. not 'ordinal')
						// 'a' & 't' are ignored (not sure of their relevance to English numbering)
						def.presentation1 = presMod[:len(presMod)-1]
					} else {
						def.presentation1 = presMod
						//TODO validate the first presentation modifier - it's either N, n, Nn or it passes analyseIntegerPicture, (JavaScript: datetime.js line 582)
						// doesn't use ] as grouping separator, and if grouping separator is , then must have width modifier
					}
				} else {
					// no presentation modifier specified - apply the default;
					def.presentation1 = defaultPresentationModifiers[def.component]
				}
				if def.presentation1 == "" {
					// unknown component specifier
					return nil, &JSONataError{
						Code:  "D3132",
						Value: def.component,
					}
				}
				if len(def.presentation1) > 0 && def.presentation1[0] == 'n' {
					def.names = tcase.LOWER
				} else if len(def.presentation1) > 0 && def.presentation1[0] == 'N' {
					if len(def.presentation1) > 1 && def.presentation1[1] == 'n' {
						def.names = tcase.TITLE
					} else {
						def.names = tcase.UPPER
					}
				} else if strings.ContainsAny(def.component, "YMDdFWwXxHhmsf") {
					integerPattern := def.presentation1
					if def.presentation2 != "" {
						integerPattern += ";" + def.presentation2
					}
					var err error
					def.integerFormat, err = analyseIntegerPicture(integerPattern)
					if err != nil {
						return nil, err
					}
					if def.width != nil && def.width.min != nil {
						if def.integerFormat.mandatoryDigits < *def.width.min {
							def.integerFormat.mandatoryDigits = *def.width.min
						}
					}
					if def.component == "Y" {
						// §9.8.4.4
						def.n = -1
						if def.width != nil && def.width.max != nil {
							def.n = *def.width.max
							def.integerFormat.mandatoryDigits = def.n
						} else {
							w := def.integerFormat.mandatoryDigits + def.integerFormat.optionalDigits
							if w >= 2 {
								def.n = w
							}
						}
					}
					// if the previous part is also an integer with no intervening markup, then its width for parsing must be precisely defined
					if len(spec) > 0 {
						previousPart := &spec[len(spec)-1]
						if previousPart.integerFormat != nil {
							previousPart.integerFormat.parseWidth = previousPart.integerFormat.mandatoryDigits
						}
					}
				}
				if def.component == "Z" || def.component == "z" {
					var err error
					def.integerFormat, err = analyseIntegerPicture(def.presentation1)
					if err != nil {
						return nil, err
					}
				}
				spec = append(spec, def)
				start = pos + 1
			}
			pos++
		}
		addLiteral(start, pos)
		format.parts = spec
		return format, nil
	}

	days := []string{"", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	months := []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	millisInADay := int64(1000 * 60 * 60 * 24)

	startOfFirstWeek := func(ym *YearMonth) int64 {
		// ISO 8601 defines the first week of the year to be the week that contains the first Thursday
		// XPath F&O extends this same definition for the first week of a month
		// the week starts on a Monday - calculate the millis for the start of the first week
		// millis for given 1st Jan of that year (at 00:00 UTC)
		jan1 := time.Date(ym.year, time.Month(ym.month+1), 1, 0, 0, 0, 0, time.UTC)
		dayOfJan1 := int(jan1.Weekday())
		if dayOfJan1 == 0 {
			dayOfJan1 = 7
		}
		// if Jan 1 is Fri, Sat or Sun, then add the number of days (in millis) to jan1 to get the start of week 1
		if dayOfJan1 > 4 {
			return jan1.Unix()*1000 + int64(8-dayOfJan1)*millisInADay
		}
		return jan1.Unix()*1000 - int64(dayOfJan1-1)*millisInADay
	}

	yearMonth := func(year, month int) *YearMonth {
		return &YearMonth{
			year:  year,
			month: month,
		}
	}

	deltaWeeks := func(start, end int64) float64 {
		return float64(end-start)/float64(millisInADay*7) + 1
	}

	getDateTimeFragment := func(date time.Time, component string) interface{} {
		var componentValue interface{}
		switch component {
		case "Y": // year
			componentValue = date.Year()
		case "M": // month in year
			componentValue = int(date.Month())
		case "D": // day in month
			componentValue = date.Day()
		case "d": // day in year
			// millis for given date (at 00:00 UTC)
			today := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
			// millis for given 1st Jan of that year (at 00:00 UTC)
			firstJan := time.Date(date.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			componentValue = int((today.Unix()-firstJan.Unix())/86400) + 1
		case "F": // day of week
			componentValue = int(date.Weekday())
			if componentValue == 0 {
				// ISO 8601 defines days 1-7: Mon-Sun
				componentValue = 7
			}
		case "W": // week in year
			thisYear := yearMonth(date.Year(), 0)
			startOfWeek1 := startOfFirstWeek(thisYear)
			today := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).Unix() * 1000
			week := deltaWeeks(startOfWeek1, today)
			if week > 52 {
				// might be first week of the following year
				startOfFollowingYear := startOfFirstWeek(thisYear.nextYear())
				if today >= startOfFollowingYear {
					week = 1
				}
			} else if week < 1 {
				// must be end of the previous year
				startOfPreviousYear := startOfFirstWeek(thisYear.previousYear())
				week = deltaWeeks(startOfPreviousYear, today)
			}
			componentValue = int(math.Floor(week))
		case "w": // week in month
			thisMonth := yearMonth(date.Year(), int(date.Month())-1)
			startOfWeek1 := startOfFirstWeek(thisMonth)
			today := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).Unix() * 1000
			week := deltaWeeks(startOfWeek1, today)
			if week > 4 {
				// might be first week of the following month
				startOfFollowingMonth := startOfFirstWeek(thisMonth.nextMonth())
				if today >= startOfFollowingMonth {
					week = 1
				}
			} else if week < 1 {
				// must be end of the previous month
				startOfPreviousMonth := startOfFirstWeek(thisMonth.previousMonth())
				week = deltaWeeks(startOfPreviousMonth, today)
			}
			componentValue = int(math.Floor(week))
		case "X": // ISO week-numbering year
			// Extension: The F&O spec says nothing about how to access the year associated with the week-of-the-year
			// e.g. Sat 1 Jan 2005 is in the 53rd week of 2004.
			// The 'W' component specifier gives 53, but 'Y' will give 2005.
			// I propose to add 'X' as the component specifier to give the ISO week-numbering year (2004 in this example)
			thisYear := yearMonth(date.Year(), 0)
			startOfISOYear := startOfFirstWeek(thisYear)
			endOfISOYear := startOfFirstWeek(thisYear.nextYear())
			now := date.Unix() * 1000
			if now < startOfISOYear {
				componentValue = thisYear.year - 1
			} else if now >= endOfISOYear {
				componentValue = thisYear.year + 1
			} else {
				componentValue = thisYear.year
			}
		case "x": // ISO week-numbering month
			// Extension: The F&O spec says nothing about how to access the month associated with the week-of-the-month
			// e.g. Sat 1 Jan 2005 is in the 5th week of December 2004.
			// The 'w' component specifier gives 5, but 'W' will give January and 'Y' will give 2005.
			// I propose to add 'x' as the component specifier to give the 'week-numbering' month (December in this example)
			thisMonth := yearMonth(date.Year(), int(date.Month())-1)
			startOfISOMonth := startOfFirstWeek(thisMonth)
			nextMonth := thisMonth.nextMonth()
			endOfISOMonth := startOfFirstWeek(nextMonth)
			now := date.Unix() * 1000
			if now < startOfISOMonth {
				componentValue = thisMonth.previousMonth().month + 1
			} else if now >= endOfISOMonth {
				componentValue = nextMonth.month + 1
			} else {
				componentValue = thisMonth.month + 1
			}
		case "H": // hour in day (24 hours)
			componentValue = date.Hour()
		case "h": // hour in half-day (12 hours)
			componentValue = date.Hour()
			componentValue = componentValue.(int) % 12
			if componentValue == 0 {
				componentValue = 12
			}
		case "P": // am/pm marker
			if date.Hour() >= 12 {
				componentValue = "pm"
			} else {
				componentValue = "am"
			}
		case "m": // minute in hour
			componentValue = date.Minute()
		case "s": // second in minute
			componentValue = date.Second()
		case "f": // fractional seconds
			componentValue = date.Nanosecond() / 1000000
		case "Z", "z": // timezone
			// since the date object is constructed from epoch millis, the TZ component is always be UTC.
			break
		case "C": // calendar name
			componentValue = "ISO"
		case "E": // era
			componentValue = "ISO"
		}
		return componentValue
	}

	var iso8601Spec *DateTimeFormat

	/**
	 * formats the date/time as specified by the XPath fn:format-dateTime function
	 * @param {int64} millis - the timestamp to be formatted, in millis since the epoch
	 * @param {string} picture - the picture string that specifies the format
	 * @param {string} timezone - the timezone to use
	 * @returns {string} - the formatted timestamp
	 * JavaScript: datetime.js line 834
	 */
	formatDateTime := func(millis int64, picture, timezone string) (string, error) {
		offsetHours := 0
		offsetMinutes := 0

		if timezone != "" {
			// Try to parse as a timezone name first
			if location, err := time.LoadLocation(timezone); err == nil {
				// Convert millis to time in the specified timezone to get offset
				t := time.Unix(millis/1000, (millis%1000)*1000000)
				_, offset := t.In(location).Zone()
				offsetHours = offset / 3600
				offsetMinutes = (offset % 3600) / 60
			} else {
				// Try parsing as numeric offset like +0500 or -0800
				offset, err := strconv.Atoi(timezone)
				if err != nil {
					// Not a valid timezone name or offset
					return "", fmt.Errorf("invalid timezone: %s", timezone)
				}
				offsetHours = offset / 100
				offsetMinutes = offset % 100
			}
		}

		formatComponent := func(date time.Time, markerSpec DateTimePart) (string, error) {
			componentValue := getDateTimeFragment(date, markerSpec.component)

			// §9.8.4.3 Formatting Integer-Valued Date/Time Components
			if strings.ContainsAny("YMDdFWwXxHhms", markerSpec.component) {
				if markerSpec.component == "Y" {
					// §9.8.4.4 Formatting the Year Component
					if markerSpec.n != -1 {
						if intVal, ok := componentValue.(int); ok {
							componentValue = intVal % int(math.Pow(10, float64(markerSpec.n)))
						}
					}
				}
				if markerSpec.names != "" {
					if markerSpec.component == "M" || markerSpec.component == "x" {
						if idx, ok := componentValue.(int); ok && idx > 0 && idx <= len(months) {
							componentValue = months[idx-1]
						}
					} else if markerSpec.component == "F" {
						if idx, ok := componentValue.(int); ok && idx > 0 && idx < len(days) {
							componentValue = days[idx]
						}
					} else {
						return "", &JSONataError{
							Code:  "D3133",
							Value: markerSpec.component,
						}
					}
					if strVal, ok := componentValue.(string); ok {
						if markerSpec.names == tcase.UPPER {
							componentValue = strings.ToUpper(strVal)
						} else if markerSpec.names == tcase.LOWER {
							componentValue = strings.ToLower(strVal)
						}
						if markerSpec.width != nil && markerSpec.width.max != nil && len(strVal) > *markerSpec.width.max {
							componentValue = strVal[:*markerSpec.width.max]
						}
					}
				} else {
					if intVal, ok := componentValue.(int); ok {
						formatted, err := _formatInteger(intVal, markerSpec.integerFormat)
						if err != nil {
							return "", err
						}
						componentValue = formatted
					}
				}
			} else if markerSpec.component == "f" {
				// TODO §9.8.4.5 Formatting Fractional Seconds (JavaScript: datetime.js line 880)
				if intVal, ok := componentValue.(int); ok {
					formatted, err := _formatInteger(intVal, markerSpec.integerFormat)
					if err != nil {
						return "", err
					}
					componentValue = formatted
				}
			} else if markerSpec.component == "Z" || markerSpec.component == "z" {
				// §9.8.4.6 Formatting timezones
				offset := offsetHours*100 + offsetMinutes
				var formatted string
				if markerSpec.integerFormat.regular {
					f, err := _formatInteger(offset, markerSpec.integerFormat)
					if err != nil {
						return "", err
					}
					formatted = f
				} else {
					numDigits := markerSpec.integerFormat.mandatoryDigits
					if numDigits == 1 || numDigits == 2 {
						f, err := _formatInteger(offsetHours, markerSpec.integerFormat)
						if err != nil {
							return "", err
						}
						formatted = f
						if offsetMinutes != 0 {
							minF, err := formatInteger(offsetMinutes, "00")
							if err != nil {
								return "", err
							}
							formatted += ":" + minF
						}
					} else if numDigits == 3 || numDigits == 4 {
						f, err := _formatInteger(offset, markerSpec.integerFormat)
						if err != nil {
							return "", err
						}
						formatted = f
					} else {
						return "", &JSONataError{
							Code:  "D3134",
							Value: numDigits,
						}
					}
				}
				if offset >= 0 {
					formatted = "+" + formatted
				}
				if markerSpec.component == "z" {
					formatted = "GMT" + formatted
				}
				if offset == 0 && markerSpec.presentation2 == "t" {
					formatted = "Z"
				}
				componentValue = formatted
			} else if markerSpec.component == "P" {
				// §9.8.4.7 Formatting Other Components
				// Formatting P for am/pm
				// getDateTimeFragment() always returns am/pm lower case so check for UPPER here
				if strVal, ok := componentValue.(string); ok {
					if markerSpec.names == tcase.UPPER {
						componentValue = strings.ToUpper(strVal)
					}
				}
			}
			if strVal, ok := componentValue.(string); ok {
				return strVal, nil
			}
			return fmt.Sprintf("%v", componentValue), nil
		}

		var formatSpec *DateTimeFormat
		if picture == "" {
			// default to ISO 8601 format
			if iso8601Spec == nil {
				var err error
				iso8601Spec, err = analyseDateTimePicture("[Y0001]-[M01]-[D01]T[H01]:[m01]:[s01].[f001][Z01:01t]")
				if err != nil {
					return "", err
				}
			}
			formatSpec = iso8601Spec
		} else {
			var err error
			formatSpec, err = analyseDateTimePicture(picture)
			if err != nil {
				return "", err
			}
		}

		offsetMillis := int64((60*offsetHours + offsetMinutes) * 60 * 1000)
		dateTime := time.Unix((millis+offsetMillis)/1000, ((millis+offsetMillis)%1000)*1000000).UTC()

		result := ""
		for _, part := range formatSpec.parts {
			if part.type_ == "literal" {
				result += part.value
			} else {
				comp, err := formatComponent(dateTime, part)
				if err != nil {
					return "", err
				}
				result += comp
			}
		}

		return result, nil
	}

	/**
	 * Generate a regex to parse integers or timestamps
	 * @param {interface{}} formatSpec - object representing the format
	 * @returns {*Matcher} - regex
	 */
	generateRegex = func(formatSpec interface{}) (*Matcher, error) {
		matcher := &Matcher{}
		switch spec := formatSpec.(type) {
		case *DateTimeFormat:
			matcher.type_ = "datetime"
			matcher.parts = []MatcherPart{}
			for _, part := range spec.parts {
				res := MatcherPart{}
				if part.type_ == "literal" {
					res.regex = regexp.QuoteMeta(part.value)
				} else if part.component == "Z" || part.component == "z" {
					// timezone
					var separator *GroupingSeparator
					if !part.integerFormat.regular && len(part.integerFormat.groupingSeparatorsList) > 0 {
						separator = &part.integerFormat.groupingSeparators
					}
					res.regex = ""
					if part.component == "z" {
						res.regex = "GMT"
					}
					res.regex += "[-+][0-9]+"
					if separator != nil {
						res.regex += regexp.QuoteMeta(separator.character) + "[0-9]+"
					} else {
						// Check for colon separator in standard timezone formats
						res.regex = ""
						if part.component == "z" {
							res.regex = "GMT"
						}
						res.regex += "[-+][0-9]{1,2}:?[0-9]{0,2}"
					}
					res.parse = func(value string) interface{} {
						if part.component == "z" {
							value = value[3:] // remove the leading GMT
						}
						offsetHours := 0
						offsetMinutes := 0
						if separator != nil {
							idx := strings.Index(value, separator.character)
							offsetHours, _ = strconv.Atoi(value[:idx])
							offsetMinutes, _ = strconv.Atoi(value[idx+1:])
						} else {
							// Check for colon separator
							if colonIdx := strings.Index(value, ":"); colonIdx != -1 {
								offsetHours, _ = strconv.Atoi(value[:colonIdx])
								offsetMinutes, _ = strconv.Atoi(value[colonIdx+1:])
							} else {
								// depends on number of digits
								numdigits := len(value) - 1
								if numdigits <= 2 {
									// just hour offset
									offsetHours, _ = strconv.Atoi(value)
								} else {
									offsetHours, _ = strconv.Atoi(value[:3])
									offsetMinutes, _ = strconv.Atoi(value[3:])
								}
							}
						}
						return offsetHours*60 + offsetMinutes
					}
				} else if part.integerFormat != nil {
					intMatcher, err := generateRegex(part.integerFormat)
					if err != nil {
						return nil, err
					}
					res.regex = intMatcher.regex
					res.parse = intMatcher.parse
				} else {
					// must be a month or day name
					res.regex = "[a-zA-Z]+"
					lookup := make(map[string]int)
					if part.component == "M" || part.component == "x" {
						// months
						for index, name := range months {
							if part.width != nil && part.width.max != nil {
								maxLen := *part.width.max
								if len(name) > maxLen {
									lookup[name[:maxLen]] = index + 1
								} else {
									lookup[name] = index + 1
								}
							} else {
								lookup[name] = index + 1
							}
						}
					} else if part.component == "F" {
						// days
						for index, name := range days {
							if index > 0 {
								if part.width != nil && part.width.max != nil {
									maxLen := *part.width.max
									if len(name) > maxLen {
										lookup[name[:maxLen]] = index
									} else {
										lookup[name] = index
									}
								} else {
									lookup[name] = index
								}
							}
						}
					} else if part.component == "P" {
						lookup = map[string]int{"am": 0, "AM": 0, "pm": 1, "PM": 1}
					} else {
						// unsupported 'name' option for this component
						return nil, &JSONataError{
							Code:  "D3133",
							Value: part.component,
						}
					}
					res.parse = func(value string) interface{} {
						return lookup[value]
					}
				}
				res.component = part.component
				matcher.parts = append(matcher.parts, res)
			}
		case *IntegerFormat: // type === 'integer'
			matcher.type_ = "integer"
			isUpper := spec.case_ == tcase.UPPER

			switch spec.primary {
			case formats.LETTERS:
				if isUpper {
					matcher.regex = "[A-Z]+"
				} else {
					matcher.regex = "[a-z]+"
				}
				matcher.parse = func(value string) interface{} {
					if isUpper {
						return lettersToDecimal(value, 'A')
					}
					return lettersToDecimal(value, 'a')
				}
			case formats.ROMAN:
				if isUpper {
					matcher.regex = "[MDCLXVI]+"
				} else {
					matcher.regex = "[mdclxvi]+"
				}
				matcher.parse = func(value string) interface{} {
					if isUpper {
						return romanToDecimal(value)
					}
					return romanToDecimal(strings.ToUpper(value))
				}
			case formats.WORDS:
				wordKeys := []string{}
				for k := range wordValues {
					wordKeys = append(wordKeys, k)
				}
				wordKeys = append(wordKeys, "and", "[\\-, ]")
				matcher.regex = "(?:" + strings.Join(wordKeys, "|") + ")+"
				matcher.parse = func(value string) interface{} {
					return wordsToNumber(strings.ToLower(value))
				}
			case formats.DECIMAL:
				matcher.regex = "[0-9]"
				if spec.parseWidth > 0 {
					matcher.regex += fmt.Sprintf("{%d}", spec.parseWidth)
				} else {
					matcher.regex += "+"
				}
				if spec.ordinal {
					// ordinals
					matcher.regex += "(?:th|st|nd|rd)"
				}
				matcher.parse = func(value string) interface{} {
					digits := value
					if spec.ordinal {
						// strip off the suffix
						digits = value[:len(value)-2]
					}
					// strip out the separators
					if spec.regular {
						digits = strings.ReplaceAll(digits, ",", "")
					} else {
						for _, sep := range spec.groupingSeparatorsList {
							digits = strings.ReplaceAll(digits, sep.character, "")
						}
					}
					if spec.zeroCode != 0x30 {
						// apply offset
						newDigits := []rune{}
						for _, char := range digits {
							newDigits = append(newDigits, char-rune(spec.zeroCode)+0x30)
						}
						digits = string(newDigits)
					}
					result, err := strconv.Atoi(digits)
					if err != nil {
						return 0
					}
					return result
				}
			case formats.SEQUENCE:
				return nil, &JSONataError{
					Code:  "D3130",
					Value: spec.token,
				}
			}
		}
		return matcher, nil
	}

	/**
	 * parse a string containing an integer as specified by the picture string
	 * @param {string} value - the string to parse
	 * @param {string} picture - the picture string
	 * @returns {int} - the parsed number
	 * JavaScript: datetime.js line 1111
	 */
	parseInteger := func(value, picture string) (interface{}, error) {
		if value == "" {
			return float64(0), nil
		}

		formatSpec, err := analyseIntegerPicture(picture)
		if err != nil {
			return nil, err
		}
		matchSpec, err := generateRegex(formatSpec)
		if err != nil {
			return nil, err
		}
		//fullRegex := "^" + matchSpec.regex + "$"
		//matcher := regexp.MustCompile(fullRegex)
		// TODO validate input based on the matcher regex (JavaScript: datetime.js line 1120)
		result := matchSpec.parse(value)
		if result == nil {
			return nil, fmt.Errorf("failed to parse integer")
		}

		// Check if the result is within safe integer range
		switch v := result.(type) {
		case int:
			return float64(v), nil
		case float64:
			// Check if the number is too large for accurate representation
			if v > 9007199254740992 { // JavaScript's MAX_SAFE_INTEGER
				// For very large numbers like 1e46, we need to check if we're parsing words
				if picture == "w" || picture == "W" || strings.Contains(picture, "w") || strings.Contains(picture, "W") {
					// Return error for numbers too large to parse from words
					return nil, &JSONataError{
						Code:    "D3100",
						Message: "Number is too large to parse from words",
						Value:   value,
					}
				}
			}
			return v, nil
		default:
			return float64(0), fmt.Errorf("unexpected parse result type: %T", result)
		}
	}

	/**
	 * parse a string containing a timestamp as specified by the picture string
	 * @param {string} timestamp - the string to parse
	 * @param {string} picture - the picture string
	 * @param {*Environment} env - the environment
	 * @returns {int64} - the parsed timestamp in millis since the epoch
	 * JavaScript: datetime.js line 1131
	 */
	parseDateTime := func(timestamp, picture string, env *Environment) (int64, error) {
		formatSpec, err := analyseDateTimePicture(picture)
		if err != nil {
			return 0, err
		}
		matchSpec, err := generateRegex(formatSpec)
		if err != nil {
			return 0, err
		}
		regexParts := []string{}
		for _, part := range matchSpec.parts {
			regexParts = append(regexParts, "("+part.regex+")")
		}
		fullRegex := "^" + strings.Join(regexParts, "") + "$"

		matcher := regexp.MustCompile("(?i)" + fullRegex) // TODO can cache this against the picture (JavaScript: datetime.js line 1136)
		info := matcher.FindStringSubmatch(timestamp)
		if info != nil {
			// validate what we've just parsed - do we have enough information to create a timestamp?
			// rules:
			// The date is specified by one of:
			//    {Y, M, D}    (dateA)
			// or {Y, d}       (dateB)
			// or {Y, x, w, F} (dateC)
			// or {X, W, F}    (dateD)
			// The time is specified by one of:
			//    {H, m, s, f}    (timeA)
			// or {P, h, m, s, f} (timeB)
			// All sets can have an optional Z
			// To create a timestamp (epoch millis) we need both date and time, but we can default missing
			// information according to the following rules:
			// - line up one combination of the above from date, and one from time, most significant value (MSV) to least significant (LSV
			// - for the values that have been captured, if there are any gaps between MSV and LSV, then throw an error
			//     (e.g.) if hour and seconds, but not minutes is given - throw
			//     (e.g.) if month, hour and minutes, but not day-of-month is given - throw
			// - anything right of the LSV should be defaulted to zero
			//     (e.g.) if hour and minutes given, default seconds and fractional seconds to zero
			//     (e.g.) if date only given, default the time to 0:00:00.000 (midnight)
			// - anything left of the MSV should be defaulted to the value of that component returned by $now()
			//     (e.g.) if time only given, default the date to today
			//     (e.g.) if month and date given, default to this year (and midnight, by previous rule)
			//   -- default values for X, x, W, w, F will be derived from the values returned by $now()

			// implement the above rules
			// determine which of the above date/time combinations we have by using bit masks

			//        Y X M x W w d D F P H h m s f Z
			// dateA  1 0 1 0 0 0 0 1 ?                     0 - must not appear
			// dateB  1 0 0 0 0 0 1 0 ?                     1 - can appear - relevant
			// dateC  0 1 0 1 0 1 0 0 1                     ? - can appear - ignored
			// dateD  0 1 0 0 1 0 0 0 1
			// timeA                    0 1 0 1 1 1
			// timeB                    1 0 1 1 1 1

			// create bitmasks based on the above
			//    date mask             YXMxWwdD
			dmA := 161 // binary 10100001
			dmB := 130 // binary 10000010
			dmC := 84  // binary 01010100
			dmD := 72  // binary 01001000
			//    time mask             PHhmsf
			tmA := 23 // binary 010111
			tmB := 47 // binary 101111

			components := make(map[string]interface{})
			for i := 1; i < len(info); i++ {
				mpart := matchSpec.parts[i-1]
				if mpart.parse != nil {
					components[mpart.component] = mpart.parse(info[i])
				}
			}

			if len(components) == 0 {
				// nothing specified - return undefined
				return 0, &JSONataError{Code: "PARSE_FAILED"}
			}

			mask := 0

			shift := func(bit bool) {
				mask <<= 1
				if bit {
					mask += 1
				}
			}

			isType := func(type_ int) bool {
				// shouldn't match any 0's, must match at least one 1
				return (^type_&mask) == 0 && (type_&mask) != 0
			}

			for _, part := range []string{"Y", "X", "M", "x", "W", "w", "d", "D"} {
				_, exists := components[part]
				shift(exists)
			}

			dateA := isType(dmA)
			dateB := !dateA && isType(dmB)
			dateC := isType(dmC)
			dateD := !dateC && isType(dmD)

			mask = 0
			for _, part := range []string{"P", "H", "h", "m", "s", "f"} {
				_, exists := components[part]
				shift(exists)
			}

			timeA := isType(tmA)
			timeB := !timeA && isType(tmB)

			// should only be zero or one date type and zero or one time type

			var dateComps string
			if dateB {
				dateComps = "YD"
			} else if dateC {
				dateComps = "XxwF"
			} else if dateD {
				dateComps = "XWF"
			} else {
				dateComps = "YMD"
			}
			var timeComps string
			if timeB {
				timeComps = "Phmsf"
			} else {
				timeComps = "Hmsf"
			}

			comps := dateComps + timeComps

			// step through the candidate parts from most significant to least significant
			// default the most significant unspecified parts to current timestamp component
			// default the least significant unspecified parts to zero
			// if any gaps in between the specified parts, throw an error

			now := time.Unix(env.timestamp/1000, (env.timestamp%1000)*1000000) // must get the fixed timestamp from jsonata

			startSpecified := false
			endSpecified := false
			for _, part := range comps {
				if _, exists := components[string(part)]; !exists {
					if startSpecified {
						// past the specified block - default to zero
						if strings.ContainsRune("MDd", part) {
							components[string(part)] = 1
						} else {
							components[string(part)] = 0
						}
						endSpecified = true
					} else {
						// haven't hit the specified block yet, default to current timestamp
						components[string(part)] = getDateTimeFragment(now, string(part))
					}
				} else {
					startSpecified = true
					if endSpecified {
						return 0, &JSONataError{
							Code: "D3136",
						}
					}
				}
			}

			// validate and fill in components
			getInt := func(key string) int {
				if val, ok := components[key].(int); ok {
					return val
				}
				return 0
			}

			if getInt("M") > 0 {
				components["M"] = getInt("M") - 1 // Date.UTC requires a zero-indexed month
			} else {
				components["M"] = 0 // default to January
			}
			if dateB {
				// millis for given 1st Jan of that year (at 00:00 UTC)
				firstJan := time.Date(getInt("Y"), 1, 1, 0, 0, 0, 0, time.UTC)
				offsetMillis := (getInt("d") - 1) * 24 * 60 * 60 * 1000
				derivedDate := time.Unix(firstJan.Unix()+int64(offsetMillis/1000), 0).UTC()
				components["M"] = int(derivedDate.Month()) - 1
				components["D"] = derivedDate.Day()
			}
			if dateC {
				// TODO implement this (JavaScript: datetime.js line 1274)
				// parsing this format not currently supported
				return 0, &JSONataError{
					Code: "D3136",
				}
			}
			if dateD {
				// TODO implement this (JavaScript: datetime.js line 1281)
				// parsing this format (ISO week date) not currently supported
				return 0, &JSONataError{
					Code: "D3136",
				}
			}
			if timeB {
				// 12hr to 24hr
				h := getInt("h")
				if h == 12 {
					components["H"] = 0
				} else {
					components["H"] = h
				}
				if getInt("P") == 1 {
					components["H"] = getInt("H") + 12
				}
			}

			date := time.Date(getInt("Y"), time.Month(getInt("M")+1), getInt("D"),
				getInt("H"), getInt("m"), getInt("s"), getInt("f")*1000000, time.UTC)
			millis := date.UnixNano() / 1000000
			if z := getInt("Z"); z != 0 {
				// adjust for timezone
				millis -= int64(z * 60 * 1000)
			} else if z := getInt("z"); z != 0 {
				// adjust for timezone
				millis -= int64(z * 60 * 1000)
			}
			return millis, nil
		}
		// JavaScript returns undefined when parsing fails
		return 0, &JSONataError{Code: "PARSE_FAILED"}
	}

	// Regular expression to match an ISO 8601 formatted timestamp
	iso8601regex := regexp.MustCompile(`^\d{4}(-[01]\d)*(-[0-3]\d)*(T[0-2]\d:[0-5]\d:[0-5]\d)*(\.\d+)?([+-][0-2]\d:?[0-5]\d|Z)?$`)

	/**
	 * Converts an ISO 8601 timestamp to milliseconds since the epoch
	 *
	 * @param {string} timestamp - the timestamp to be converted
	 * @param {string} picture - the picture string defining the format of the timestamp (defaults to ISO 8601)
	 * @param {*Environment} env - the environment
	 * @returns {int64} - milliseconds since the epoch
	 * JavaScript: datetime.js line 1314
	 */
	toMillis := func(timestamp, picture string, env *Environment) (int64, error) {
		// undefined inputs always return undefined
		if timestamp == "" {
			return 0, nil
		}

		if picture == "" {
			if !iso8601regex.MatchString(timestamp) {
				return 0, &JSONataError{
					Code:  "D3110",
					Value: timestamp,
				}
			}

			// JavaScript's Date.parse is very flexible. We need to handle various formats
			var t time.Time
			var err error

			// Try various ISO 8601 formats that JavaScript accepts
			// First try formats with explicit timezone
			formats := []string{
				time.RFC3339,                    // 2006-01-02T15:04:05Z07:00
				time.RFC3339Nano,                // 2006-01-02T15:04:05.999999999Z07:00
				"2006-01-02T15:04:05.999Z07:00", // With milliseconds
				"2006-01-02T15:04:05.999+0000",  // With milliseconds and +0000 timezone
				"2006-01-02T15:04:05+0000",      // Without milliseconds and +0000 timezone
			}

			for _, format := range formats {
				t, err = time.Parse(format, timestamp)
				if err == nil {
					break
				}
			}

			// If no timezone specified, handle based on JavaScript Date.parse behavior
			if err != nil {
				// Try datetime without timezone first (parsed as local time)
				t, err = time.ParseInLocation("2006-01-02T15:04:05", timestamp, time.Local)

				// If that fails, try date-only formats (parsed as UTC)
				if err != nil {
					utcFormats := []string{
						"2006-01-02", // Date only
						"2006",       // Year only
					}

					for _, format := range utcFormats {
						t, err = time.Parse(format, timestamp)
						if err == nil {
							break
						}
					}
				}
			}

			if err != nil {
				return 0, err
			}

			// Convert to milliseconds, preserving the millisecond precision
			millis := t.UnixNano() / 1000000
			return millis, nil
		} else {
			return parseDateTime(timestamp, picture, env)
		}
	}

	/**
	 * Converts milliseconds since the epoch to an ISO 8601 timestamp
	 * @param {int64} millis - milliseconds since the epoch to be converted
	 * @param {string} picture - the picture string defining the format of the timestamp (defaults to ISO 8601)
	 * @param {string} timezone - the timezone to format the timestamp in (defaults to UTC)
	 * @returns {string} - the formatted timestamp
	 * JavaScript: datetime.js line 1342
	 */
	fromMillis := func(millis int64, picture, timezone string) (string, error) {
		// undefined inputs always return undefined
		if millis == 0 {
			return "", nil
		}

		// JavaScript defaults to UTC when timezone is empty
		if timezone == "" {
			timezone = "UTC"
		}

		return formatDateTime(millis, picture, timezone)
	}

	return &DateTimeModule{
		formatInteger: formatInteger,
		parseInteger:  parseInteger,
		fromMillis:    fromMillis,
		toMillis:      toMillis,
	}
}

// DateTimeModule contains date/time formatting functions
type DateTimeModule struct {
	formatInteger func(interface{}, string) (string, error)
	parseInteger  func(string, string) (interface{}, error)
	fromMillis    func(int64, string, string) (string, error)
	toMillis      func(string, string, *Environment) (int64, error)
}

// Helper types for the date/time module

// IntegerFormat represents analyzed integer picture
type IntegerFormat struct {
	type_                  string
	primary                string
	case_                  string
	ordinal                bool
	regular                bool
	zeroCode               int
	mandatoryDigits        int
	optionalDigits         int
	groupingSeparators     GroupingSeparator
	groupingSeparatorsList []GroupingSeparator
	token                  string
	parseWidth             int
}

// GroupingSeparator represents a grouping separator
type GroupingSeparator struct {
	position  int
	character string
}

// DateTimeFormat represents analyzed datetime picture
type DateTimeFormat struct {
	type_ string
	parts []DateTimePart
}

// DateTimePart represents a part of datetime format
type DateTimePart struct {
	type_         string
	value         string
	component     string
	presentation1 string
	presentation2 string
	ordinal       bool
	names         string
	integerFormat *IntegerFormat
	width         *WidthDef
	n             int
}

// WidthDef represents width definition
type WidthDef struct {
	min *int
	max *int
}

// YearMonth helper struct
type YearMonth struct {
	year  int
	month int
}

func (ym *YearMonth) nextMonth() *YearMonth {
	if ym.month == 11 {
		return &YearMonth{year: ym.year + 1, month: 0}
	}
	return &YearMonth{year: ym.year, month: ym.month + 1}
}

func (ym *YearMonth) previousMonth() *YearMonth {
	if ym.month == 0 {
		return &YearMonth{year: ym.year - 1, month: 11}
	}
	return &YearMonth{year: ym.year, month: ym.month - 1}
}

func (ym *YearMonth) nextYear() *YearMonth {
	return &YearMonth{year: ym.year + 1, month: ym.month}
}

func (ym *YearMonth) previousYear() *YearMonth {
	return &YearMonth{year: ym.year - 1, month: ym.month}
}

// Matcher represents regex matcher with parse function
type Matcher struct {
	type_ string
	regex string
	parts []MatcherPart
	parse func(string) interface{}
}

// MatcherPart represents a part of a matcher
type MatcherPart struct {
	regex     string
	component string
	parse     func(string) interface{}
}

// Environment represents the JSONata environment (placeholder)
type Environment struct {
	timestamp int64
}
