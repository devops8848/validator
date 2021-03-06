package validator

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stvp/rollbar"
	"gopkg.in/go-playground/validator.v8"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrorInternalError = errors.New("whoops something went wrong")
)

// This method is for uppercase first letter of word
func UcFirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}

//This method is to change lowercase word
func LcFirst(str string) string {
	return strings.ToLower(str)
}


// This method is to split camelcase letter
func Split(src string) string {
	// don't split invalid utf8
	if !utf8.ValidString(src) {
		return src
	}
	var entries []string
	var runes [][]rune
	lastClass := 0
	class := 0
	// split into fields based on class of unicode character
	for _, r := range src {
		switch true {
		case unicode.IsLower(r):
			class = 1
		case unicode.IsUpper(r):
			class = 2
		case unicode.IsDigit(r):
			class = 3
		default:
			class = 4
		}
		if class == lastClass {
			runes[len(runes)-1] = append(runes[len(runes)-1], r)
		} else {
			runes = append(runes, []rune{r})
		}
		lastClass = class
	}

	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsUpper(runes[i][0]) && unicode.IsLower(runes[i+1][0]) {
			runes[i+1] = append([]rune{runes[i][len(runes[i])-1]}, runes[i+1]...)
			runes[i] = runes[i][:len(runes[i])-1]
		}
	}
	// construct []string from results
	for _, s := range runes {
		if len(s) > 0 {
			entries = append(entries, string(s))
		}
	}

	for index, word := range entries {
		if index == 0 {
			entries[index] = UcFirst(word)
		} else {
			entries[index] = LcFirst(word)
		}
	}
	justString := strings.Join(entries, " ")
	return justString
}


// Extra validation struct
type ExtraValidation struct{
	Tag string
	Message string

}

// Initializing default Validation object
var ValidationObject = []ExtraValidation{
{Tag: "required", Message:"%s is required!"},
{Tag: "max", Message:"%s cannot be longer than %s!"},
{Tag: "min", Message:"%s must be minimum %s characters!"},
{Tag: "email", Message:"Invalid email format!"},
{Tag: "len", Message:"%s must be %s characters long!"},
}

// This method is for registering new validator
func MakeExtraValidation(v []ExtraValidation) {
	for _, vObj := range v {
		ValidationObject = append(ValidationObject, vObj)
	}

}

// Check if param is involved in valdation message
func checkOccurance(msg string, word string, param string)(ans string) {
	reg := regexp.MustCompile("%s")

	matches := reg.FindAllStringIndex(msg, -1)
	if len(matches) == 2 {
		ans = fmt.Sprintf(msg, word, param)
	} else {
		ans = fmt.Sprintf(msg, word)
	}
	return
}


// This method changes FieldError to string
func ValidationErrorToText(e *validator.FieldError) string {
	word := Split(e.Field)
	var result string
	for _, validate := range ValidationObject {
		if e.Tag == validate.Tag {
				result =  checkOccurance(validate.Message, word, e.Param)
		}
	}
	if result == "" {
		result =  fmt.Sprintf("%s is not valid", word)
	}

	return result

}

// This method collects all errors and submits them to Rollbar
func Errors() gin.HandlerFunc {

	return func(c *gin.Context) {
		// Only run if there are some errors to handle
		c.Next()
		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				// Find out what type of error it is
				switch e.Type {
				case gin.ErrorTypePublic:
					if !c.Writer.Written() {
						c.JSON(c.Writer.Status(), gin.H{"Error": e.Error()})
					}
				case gin.ErrorTypeBind:
					errs := e.Err.(validator.ValidationErrors)
					list := make(map[string]string)

					for _, err := range errs {
						list[strings.ToLower(err.Field)] = ValidationErrorToText(err)
					}
					status := http.StatusUnprocessableEntity
					c.JSON(status, gin.H{"error": list})
					c.Abort()
					return
				default:
					// Log all other errors
					rollbar.RequestError(rollbar.ERR, c.Request, e.Err)
				}

			}
			// If there was no public or bind error, display default 500 message
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{"error": ErrorInternalError.Error()})
			}
		}
	}
}