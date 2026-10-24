package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func OK(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, APIResponse{Success: true, Message: message, Data: data})
}

func Fail(c *gin.Context, code int, err string) {
	c.JSON(code, APIResponse{Success: false, Error: err})
}

// BindError answers a failed ShouldBindJSON with a readable message instead of the raw
// validator text ("Key: 'x.Amount' Error:Field validation for ...").
func BindError(c *gin.Context, err error) {
	status, msg := ClassifyBindError(err)
	Fail(c, status, msg)
}

// ClassifyBindError maps a binding error to an HTTP status and a client-safe message.
func ClassifyBindError(err error) (int, string) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return http.StatusRequestEntityTooLarge, "request body is too large"
	}
	if errors.Is(err, io.EOF) {
		return http.StatusBadRequest, "request body is empty"
	}

	var validation validator.ValidationErrors
	if errors.As(err, &validation) {
		parts := make([]string, 0, len(validation))
		for _, fe := range validation {
			parts = append(parts, describe(fe))
		}
		return http.StatusBadRequest, strings.Join(parts, "; ")
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "body"
		}
		return http.StatusBadRequest, fmt.Sprintf("%s must be %s", field, kindName(typeErr.Type))
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || errors.Is(err, io.ErrUnexpectedEOF) {
		return http.StatusBadRequest, "request body is not valid JSON"
	}
	return http.StatusBadRequest, "invalid request body"
}

func kindName(t reflect.Type) string {
	if t == nil {
		return "a different type"
	}
	switch t.Kind() {
	case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		return "a number"
	case reflect.String:
		return "text"
	case reflect.Bool:
		return "true or false"
	default:
		return "a different type"
	}
}

func describe(fe validator.FieldError) string {
	name := snake(fe.Field())
	isText := fe.Kind() == reflect.String
	switch fe.Tag() {
	case "required":
		return name + " is required"
	case "email":
		return name + " must be a valid email address"
	case "min", "gte":
		if isText {
			return fmt.Sprintf("%s must be at least %s characters", name, fe.Param())
		}
		return fmt.Sprintf("%s must be at least %s", name, fe.Param())
	case "max", "lte":
		if isText {
			return fmt.Sprintf("%s must be at most %s characters", name, fe.Param())
		}
		return fmt.Sprintf("%s must be at most %s", name, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", name, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", name, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", name, strings.ReplaceAll(fe.Param(), " ", ", "))
	default:
		return name + " is invalid"
	}
}

// snake turns Go field names into API names: BusinessID -> business_id.
func snake(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
