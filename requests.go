package main

// OBJECTIVE: request-and-approval workflow for borrowing, with due-date tracking
//
// The story:
//   1. Priya finds Rahul's book and sends a REQUEST      (status: pending)
//   2. Rahul APPROVES it and a due date is set           (status: approved, book: borrowed)
//      ...or REJECTS it                                  (status: rejected)
//   3. Priya reads the book. When she gives it back,
//      Rahul marks it RETURNED                           (status: returned, book: available)
//   4. If today is past the due date, the request shows "overdue": true

import (
	"net/http"
	"time"
)

const requestColumns = "r.id, r.book_id, b.title, r.borrower_id, b.owner_id, r.status, r.requested_at, r.due_date, r.returned_at"

// scanRequest copies one database row into a BorrowRequest.
func scanRequest(s scanner) (BorrowRequest, error) {
	var q BorrowRequest
	err := s.Scan(&q.ID, &q.BookID, &q.BookTitle, &q.BorrowerID, &q.OwnerID,
		&q.Status, &q.RequestedAt, &q.DueDate, &q.ReturnedAt)
	// Overdue = approved, not returned yet, and the due date is in the past.
	// (Dates like "2026-09-28" can be compared as plain text.)
	q.Overdue = q.Status == "approved" && q.DueDate < today()
	return q, err
}

// findRequestByID gets one request.
func findRequestByID(id int) (BorrowRequest, error) {
	row := db.QueryRow("SELECT "+requestColumns+
		" FROM borrow_requests r JOIN books b ON b.id = r.book_id WHERE r.id = ?", id)
	return scanRequest(row)
}

// findRequests gets a list. `where` is a condition like "r.borrower_id = ?"
func findRequests(where string, arg int) []BorrowRequest {
	list := []BorrowRequest{}
	rows, err := db.Query("SELECT "+requestColumns+
		" FROM borrow_requests r JOIN books b ON b.id = r.book_id WHERE "+where+" ORDER BY r.id DESC", arg)
	if err != nil {
		return list
	}
	defer rows.Close()
	for rows.Next() {
		q, err := scanRequest(rows)
		if err == nil {
			list = append(list, q)
		}
	}
	return list
}

// POST /books/{id}/request  (login needed)
// "Can I borrow this book?"
func requestBookHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	bookID, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "book id must be a number")
		return
	}

	book, err := getBookByID(bookID)
	if err != nil {
		sendError(w, http.StatusNotFound, "book not found")
		return
	}
	if book.OwnerID == userID {
		sendError(w, http.StatusBadRequest, "you can't borrow your own book")
		return
	}
	if !book.ForLend {
		sendError(w, http.StatusBadRequest, "this book is not offered for lending")
		return
	}
	if book.Status != "available" {
		sendError(w, http.StatusConflict, "this book is not available right now")
		return
	}

	// Don't allow duplicate pending requests for the same book
	var count int
	db.QueryRow("SELECT COUNT(*) FROM borrow_requests WHERE book_id = ? AND borrower_id = ? AND status = 'pending'",
		bookID, userID).Scan(&count)
	if count > 0 {
		sendError(w, http.StatusConflict, "you already have a pending request for this book")
		return
	}

	result, err := db.Exec("INSERT INTO borrow_requests (book_id, borrower_id, requested_at) VALUES (?, ?, ?)",
		bookID, userID, now())
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not create request")
		return
	}

	id, _ := result.LastInsertId()
	request, _ := findRequestByID(int(id))
	sendJSON(w, http.StatusCreated, request)
}

// GET /requests/incoming  -> requests other people sent for MY books
func incomingRequestsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, findRequests("b.owner_id = ?", userID))
}

// GET /requests/outgoing  -> requests I sent (shows due dates & overdue flag)
func outgoingRequestsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, findRequests("r.borrower_id = ?", userID))
}

// POST /requests/{id}/approve  (book owner only)
// Optional body: {"days": 7}   (default is 14 days)
func approveRequestHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	id, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "request id must be a number")
		return
	}

	request, err := findRequestByID(id)
	if err != nil {
		sendError(w, http.StatusNotFound, "request not found")
		return
	}
	if request.OwnerID != userID {
		sendError(w, http.StatusForbidden, "only the book owner can approve")
		return
	}
	if request.Status != "pending" {
		sendError(w, http.StatusBadRequest, "this request is already "+request.Status)
		return
	}

	book, _ := getBookByID(request.BookID)
	if book.Status != "available" {
		sendError(w, http.StatusConflict, "book is no longer available")
		return
	}

	// How many days can the borrower keep it?
	days := 14
	var input struct {
		Days int `json:"days"`
	}
	if readJSON(r, &input) == nil && input.Days > 0 && input.Days <= 90 {
		days = input.Days
	}
	dueDate := time.Now().AddDate(0, 0, days).Format("2006-01-02")

	// Two updates that must BOTH succeed -> use a transaction.
	// (Like a bank transfer: either both happen or neither.)
	tx, err := db.Begin()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "database error")
		return
	}
	if _, err := tx.Exec("UPDATE borrow_requests SET status = 'approved', due_date = ? WHERE id = ?", dueDate, id); err != nil {
		tx.Rollback()
		sendError(w, http.StatusInternalServerError, "could not approve")
		return
	}
	if _, err := tx.Exec("UPDATE books SET status = 'borrowed' WHERE id = ?", request.BookID); err != nil {
		tx.Rollback()
		sendError(w, http.StatusInternalServerError, "could not update book")
		return
	}
	tx.Commit()

	updated, _ := findRequestByID(id)
	sendJSON(w, http.StatusOK, updated)
}

// POST /requests/{id}/reject  (book owner only)
func rejectRequestHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	id, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "request id must be a number")
		return
	}

	request, err := findRequestByID(id)
	if err != nil {
		sendError(w, http.StatusNotFound, "request not found")
		return
	}
	if request.OwnerID != userID {
		sendError(w, http.StatusForbidden, "only the book owner can reject")
		return
	}
	if request.Status != "pending" {
		sendError(w, http.StatusBadRequest, "this request is already "+request.Status)
		return
	}

	db.Exec("UPDATE borrow_requests SET status = 'rejected' WHERE id = ?", id)
	updated, _ := findRequestByID(id)
	sendJSON(w, http.StatusOK, updated)
}

// POST /requests/{id}/return  (book owner confirms they got the book back)
func returnRequestHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	id, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "request id must be a number")
		return
	}

	request, err := findRequestByID(id)
	if err != nil {
		sendError(w, http.StatusNotFound, "request not found")
		return
	}
	if request.OwnerID != userID {
		sendError(w, http.StatusForbidden, "only the book owner can confirm the return")
		return
	}
	if request.Status != "approved" {
		sendError(w, http.StatusBadRequest, "only approved (borrowed) books can be returned")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "database error")
		return
	}
	if _, err := tx.Exec("UPDATE borrow_requests SET status = 'returned', returned_at = ? WHERE id = ?", now(), id); err != nil {
		tx.Rollback()
		sendError(w, http.StatusInternalServerError, "could not mark as returned")
		return
	}
	if _, err := tx.Exec("UPDATE books SET status = 'available' WHERE id = ?", request.BookID); err != nil {
		tx.Rollback()
		sendError(w, http.StatusInternalServerError, "could not update book")
		return
	}
	tx.Commit()

	updated, _ := findRequestByID(id)
	sendJSON(w, http.StatusOK, updated)
}
