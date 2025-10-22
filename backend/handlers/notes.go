package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"arthik/models"
	"arthik/services"
	"arthik/utils"
)

// NotesHandler handles notes listing and creation
func NotesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "GET" {
		notes, err := services.LoadAllNotes()
		if err != nil {
			utils.LogAudit(ip, "admin", "GET_NOTES_FAILED", "", false)
			http.Error(w, "Failed to load notes", http.StatusInternalServerError)
			return
		}

		utils.LogAudit(ip, "admin", "GET_NOTES", "", true)
		json.NewEncoder(w).Encode(notes)
		
	} else if r.Method == "POST" {
		var note models.Note
		if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
			utils.LogAudit(ip, "admin", "ADD_NOTE_INVALID", "", false)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate and sanitize heading
		sanitizedHeading, err := utils.ValidateAndSanitize(note.Heading, 500)
		if err != nil {
			utils.LogAudit(ip, "admin", "ADD_NOTE_INVALID_HEADING", "", false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		note.Heading = sanitizedHeading

		// Validate and sanitize content
		sanitizedContent, err := utils.ValidateAndSanitize(note.Content, 2000)
		if err != nil {
			utils.LogAudit(ip, "admin", "ADD_NOTE_INVALID_CONTENT", "", false)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		note.Content = sanitizedContent

		// Generate ID and created timestamp if new note
		if note.ID == "" {
			note.ID = utils.GenerateNoteID()
			note.Created = time.Now().Format("2006-01-02 15:04:05")
		}

		// Save note
		if err := services.SaveNote(note); err != nil {
			utils.LogAudit(ip, "admin", "ADD_NOTE_FAILED", note.ID, false)
			http.Error(w, "Failed to save note", http.StatusInternalServerError)
			return
		}

		utils.LogAudit(ip, "admin", "ADD_NOTE_SUCCESS", note.ID, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}

// NoteDetailHandler handles individual note operations
func NoteDetailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ip := utils.GetClientIP(r)

	if r.Method == "DELETE" {
		id := strings.TrimPrefix(r.URL.Path, "/api/notes/")
		id, _ = url.QueryUnescape(id)
		
		if id == "" {
			http.Error(w, "Note ID required", http.StatusBadRequest)
			return
		}

		if err := services.DeleteNote(id); err != nil {
			utils.LogAudit(ip, "admin", "DELETE_NOTE_FAILED", id, false)
			http.Error(w, "Failed to delete note", http.StatusInternalServerError)
			return
		}

		utils.LogAudit(ip, "admin", "DELETE_NOTE", id, true)
		json.NewEncoder(w).Encode(models.GenericResponse{Success: true})
	}
}