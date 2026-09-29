package main

// OBJECTIVE: list, search and filter books by genre, author, price, availability

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

// The columns we always read for a book. "b" is a short name for the books table.
const bookColumns = "b.id, b.title, b.author, b.genre, b.description, b.owner_id, b.for_lend, b.for_sale, b.price, b.status"

// scanner lets scanBook work for both a single row and a row inside a loop.
type scanner interface {
	Scan(dest ...any) error
}

// scanBook copies one database row into a Book struct.
func scanBook(s scanner) (Book, error) {
	var b Book
	err := s.Scan(&b.ID, &b.Title, &b.Author, &b.Genre, &b.Description,
		&b.OwnerID, &b.ForLend, &b.ForSale, &b.Price, &b.Status)
	return b, err
}

// readBooks loops over query results and returns a list of books.
// It also closes the rows when done.
func readBooks(rows *sql.Rows) []Book {
	defer rows.Close()
	list := []Book{} // empty list (so JSON shows [] not null)
	for rows.Next() {
		b, err := scanBook(rows)
		if err == nil {
			list = append(list, b)
		}
	}
	return list
}

// getBookByID finds one book. Returns an error if it doesn't exist.
func getBookByID(id int) (Book, error) {
	row := db.QueryRow("SELECT "+bookColumns+" FROM books b WHERE b.id = ?", id)
	return scanBook(row)
}

// validateBookInput returns an error message, or "" if everything is fine.
func validateBookInput(in BookInput) string {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Author) == "" {
		return "title and author are required"
	}
	if !in.ForLend && !in.ForSale {
		return "book must be for_lend, for_sale, or both"
	}
	if in.ForSale && in.Price <= 0 {
		return "a book for sale needs a price greater than 0"
	}
	return ""
}

// GET /books
// Filters (all optional, combine as you like):
//   ?search=harry        -> matches title or author
//   ?genre=fantasy
//   ?author=rowling
//   ?min_price=100&max_price=500   (only books for sale)
//   ?available=true      -> only books that can be borrowed/bought right now
//   ?type=lend  or  ?type=sale
func getAllBooksHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// We build the SQL step by step, adding a condition for every filter used.
	query := "SELECT " + bookColumns + " FROM books b WHERE 1=1"
	args := []any{}

	if search := strings.ToLower(strings.TrimSpace(q.Get("search"))); search != "" {
		query += " AND (LOWER(b.title) LIKE ? OR LOWER(b.author) LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}
	if genre := strings.ToLower(strings.TrimSpace(q.Get("genre"))); genre != "" {
		query += " AND LOWER(b.genre) = ?"
		args = append(args, genre)
	}
	if author := strings.ToLower(strings.TrimSpace(q.Get("author"))); author != "" {
		query += " AND LOWER(b.author) LIKE ?"
		args = append(args, "%"+author+"%")
	}
	if v := q.Get("min_price"); v != "" {
		minPrice, err := strconv.ParseFloat(v, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, "min_price must be a number")
			return
		}
		query += " AND b.for_sale = 1 AND b.price >= ?"
		args = append(args, minPrice)
	}
	if v := q.Get("max_price"); v != "" {
		maxPrice, err := strconv.ParseFloat(v, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, "max_price must be a number")
			return
		}
		query += " AND b.for_sale = 1 AND b.price <= ?"
		args = append(args, maxPrice)
	}
	if q.Get("available") == "true" {
		query += " AND b.status = 'available'"
	}
	switch q.Get("type") {
	case "lend":
		query += " AND b.for_lend = 1"
	case "sale":
		query += " AND b.for_sale = 1"
	}

	query += " ORDER BY b.id DESC" // newest first

	rows, err := db.Query(query, args...)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not load books")
		return
	}
	sendJSON(w, http.StatusOK, readBooks(rows))
}

// GET /books/{id}
func getOneBookHandler(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "book id must be a number")
		return
	}
	book, err := getBookByID(id)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}
	sendJSON(w, http.StatusOK, book)
}

// POST /books  (login needed)
// Body: {"title":"Dune","author":"Frank Herbert","genre":"Sci-Fi","description":"...",
//        "for_lend":true,"for_sale":true,"price":250}
func addBookHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var input BookInput
	if err := readJSON(r, &input); err != nil {
		sendError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg := validateBookInput(input); msg != "" {
		sendError(w, http.StatusBadRequest, msg)
		return
	}
	if !input.ForSale {
		input.Price = 0 // price only matters for books that are sold
	}

	result, err := db.Exec(`INSERT INTO books
		(title, author, genre, description, owner_id, for_lend, for_sale, price)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Title, input.Author, input.Genre, input.Description,
		userID, input.ForLend, input.ForSale, input.Price)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not save book")
		return
	}

	id, _ := result.LastInsertId()
	book, _ := getBookByID(int(id))
	sendJSON(w, http.StatusCreated, book)
}

// PUT /books/{id}  (owner only, book must be available)
func updateBookHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	id, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "book id must be a number")
		return
	}

	book, err := getBookByID(id)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}
	if book.OwnerID != userID {
		sendError(w, http.StatusForbidden, "you can only edit your own books")
		return
	}
	if book.Status != "available" {
		sendError(w, http.StatusBadRequest, "you can't edit a book that is borrowed or sold")
		return
	}

	var input BookInput
	if err := readJSON(r, &input); err != nil {
		sendError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if msg := validateBookInput(input); msg != "" {
		sendError(w, http.StatusBadRequest, msg)
		return
	}
	if !input.ForSale {
		input.Price = 0
	}

	_, err = db.Exec(`UPDATE books SET title=?, author=?, genre=?, description=?,
		for_lend=?, for_sale=?, price=? WHERE id=?`,
		input.Title, input.Author, input.Genre, input.Description,
		input.ForLend, input.ForSale, input.Price, id)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not update book")
		return
	}

	updated, _ := getBookByID(id)
	sendJSON(w, http.StatusOK, updated)
}

// DELETE /books/{id}  (owner only, book must be available)
func deleteBookHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	id, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "book id must be a number")
		return
	}

	book, err := getBookByID(id)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}
	if book.OwnerID != userID {
		sendError(w, http.StatusForbidden, "you can only delete your own books")
		return
	}
	if book.Status != "available" {
		sendError(w, http.StatusBadRequest, "you can't delete a book that is borrowed or sold")
		return
	}

	db.Exec("DELETE FROM cart_items WHERE book_id = ?", id) // remove from anyone's cart
	db.Exec("DELETE FROM books WHERE id = ?", id)
	sendJSON(w, http.StatusOK, map[string]string{"message": "book deleted"})
}

// GET /my/books  -> all books I listed
func myBooksHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	rows, err := db.Query("SELECT "+bookColumns+" FROM books b WHERE b.owner_id = ? ORDER BY b.id DESC", userID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not load books")
		return
	}
	sendJSON(w, http.StatusOK, readBooks(rows))
}
