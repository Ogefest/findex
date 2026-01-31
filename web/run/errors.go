package webapp

import (
	"log"
	"net/http"
)

type errorData struct {
	Code    int
	Title   string
	Message string
	Icon    string
	Color   string
}

var errorTemplates = map[int]errorData{
	http.StatusBadRequest: {
		Code:    400,
		Title:   "Bad Request",
		Message: "The request could not be understood by the server.",
		Icon:    "x-circle",
		Color:   "warning",
	},
	http.StatusNotFound: {
		Code:    404,
		Title:   "Not Found",
		Message: "The page or file you're looking for doesn't exist or has been moved.",
		Icon:    "question-circle",
		Color:   "secondary",
	},
	http.StatusInternalServerError: {
		Code:    500,
		Title:   "Internal Server Error",
		Message: "Something went wrong on our end. Please try again later.",
		Icon:    "exclamation-triangle",
		Color:   "danger",
	},
	http.StatusServiceUnavailable: {
		Code:    503,
		Title:   "Service Unavailable",
		Message: "The service is temporarily unavailable. Please try again later.",
		Icon:    "hourglass-split",
		Color:   "warning",
	},
}

func (webapp *WebApp) renderError(w http.ResponseWriter, code int, customMessage string) {
	data, ok := errorTemplates[code]
	if !ok {
		data = errorData{
			Code:    code,
			Title:   "Error",
			Message: "An unexpected error occurred.",
			Icon:    "exclamation-circle",
			Color:   "danger",
		}
	}

	if customMessage != "" {
		data.Message = customMessage
	}

	w.WriteHeader(code)

	tmpl := webapp.TemplateCache["error.html"]
	if tmpl == nil {
		log.Printf("Error template not found, falling back to plain text")
		http.Error(w, data.Message, code)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "error.html", data); err != nil {
		log.Printf("Error rendering error template: %v", err)
		http.Error(w, data.Message, code)
	}
}

func (webapp *WebApp) notFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		webapp.renderError(w, http.StatusNotFound, "")
	}
}
