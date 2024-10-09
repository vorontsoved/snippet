package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
)

// ApiError представляет ошибки бизнес-логики
type ApiError struct {
	StatusCode int `json:"statusCode"`
	Msg        any `json:"msg"`
}

func (e ApiError) Error() string {
	return fmt.Sprintf("%d: %v", e.StatusCode, e.Msg)
}

func NewApiError(statusCode int, err error) ApiError {
	return ApiError{
		StatusCode: statusCode,
		Msg:        err.Error(),
	}
}

// InfraError представляет ошибки инфраструктуры
type InfraError struct {
	ServiceName string
	Msg         string
}

func (e InfraError) Error() string {
	return fmt.Sprintf("infrastructure error with service %s: %s", e.ServiceName, e.Msg)
}

func NewInfraError(serviceName, msg string) InfraError {
	return InfraError{
		ServiceName: serviceName,
		Msg:         msg,
	}
}

// APIFunc - обработчик API, который может вернуть ошибку
type APIFunc func(w http.ResponseWriter, r *http.Request) error

// Make оборачивает APIFunc для обработки ошибок
func Make(h APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			switch e := err.(type) {
			case ApiError:
				// Возвращаем ошибку бизнес-логики
				writeJSON(w, e.StatusCode, e)
			case InfraError:
				// Возвращаем инфраструктурную ошибку с кодом 503
				errResp := map[string]any{
					"statusCode": http.StatusServiceUnavailable,
					"msg":        "service temporarily unavailable",
				}
				writeJSON(w, http.StatusServiceUnavailable, errResp)
				// Логируем инфраструктурную ошибку с подробностями
				slog.Error("Infrastructure error", "service", e.ServiceName, "msg", e.Msg, "path", r.URL.Path)
			default:
				// Общая ошибка сервера
				errResp := map[string]any{
					"statusCode": http.StatusInternalServerError,
					"msg":        "internal server error",
				}
				writeJSON(w, http.StatusInternalServerError, errResp)
				slog.Error("Unknown error", "err", err.Error(), "path", r.URL.Path)
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

// Пример обработчика бизнес-логики
func validationErrorHandler(w http.ResponseWriter, r *http.Request) error {
	errors := map[string]string{
		"username": "username is required",
		"email":    "email is invalid",
	}
	return ApiError{
		StatusCode: http.StatusUnprocessableEntity,
		Msg:        errors,
	}
}

func dbErrorHandler(w http.ResponseWriter, r *http.Request) error {
	return NewInfraError("Database", "failed to connect to database")
}

func cacheErrorHandler(w http.ResponseWriter, r *http.Request) error {
	return NewInfraError("Cache", "failed to connect to Redis")
}

type Response struct {
	Message string `json:"message"`
}

func helloHandler(w http.ResponseWriter, r *http.Request) error {
	response := Response{Message: "Hello, World!"}
	return writeJSON(w, http.StatusOK, response)
}

func main() {
	// Регистрация маршрутов с обработчиками
	http.HandleFunc("/hello", Make(helloHandler))
	http.HandleFunc("/validationerror", Make(validationErrorHandler))
	http.HandleFunc("/dberror", Make(dbErrorHandler))
	http.HandleFunc("/cacheerror", Make(cacheErrorHandler))

	port := 4009
	fmt.Printf("Starting server on port %d...\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
