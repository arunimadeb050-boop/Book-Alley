package main

// OBJECTIVE: rating/reputation system to encourage trustworthy exchanges
//
// Rule: you can only rate someone AFTER a real exchange happened with them
// (a completed book purchase, or a returned borrow). This stops random/fake reviews.
// One review per pair of users (rating it again just updates your old review).

import (
	"net/http"
)

// hadExchangeWith checks if userA and userB ever completed a deal together
// (either direction: bought/sold, or borrowed/lent and returned).
func hadExchangeWith(userA, userB int) bool {
	var count int

	// Did they buy/sell a book together?
	db.QueryRow(`SELECT COUNT(*) FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		WHERE (o.buyer_id = ? AND oi.seller_id = ?)
		   OR (o.buyer_id = ? AND oi.seller_id = ?)`,
		userA, userB, userB, userA).Scan(&count)
	if count > 0 {
		return true
	}

	// Did they complete a borrow (status returned)?
	db.QueryRow(`SELECT COUNT(*) FROM borrow_requests r
		JOIN books b ON b.id = r.book_id
		WHERE r.status = 'returned'
		  AND ((r.borrower_id = ? AND b.owner_id = ?)
		    OR (r.borrower_id = ? AND b.owner_id = ?))`,
		userA, userB, userB, userA).Scan(&count)
	return count > 0
}

// POST /users/{id}/review  (login needed)
// Body: {"rating": 5, "comment": "Returned the book on time, great!"}
func addReviewHandler(w http.ResponseWriter, r *http.Request) {
	reviewerID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	reviewedID, err := getIDFromURL(r, "id")
	if err != nil {
		sendError(w, http.StatusBadRequest, "user id must be a number")
		return
	}
	if reviewedID == reviewerID {
		sendError(w, http.StatusBadRequest, "you can't review yourself")
		return
	}

	var input struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := readJSON(r, &input); err != nil {
		sendError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if input.Rating < 1 || input.Rating > 5 {
		sendError(w, http.StatusBadRequest, "rating must be between 1 and 5")
		return
	}

	if !hadExchangeWith(reviewerID, reviewedID) {
		sendError(w, http.StatusForbidden, "you can only review someone after a completed exchange with them")
		return
	}

	// INSERT OR REPLACE: if a review already exists for this pair, update it instead.
	_, err = db.Exec(`INSERT INTO reviews (reviewer_id, reviewed_id, rating, comment, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(reviewer_id, reviewed_id) DO UPDATE SET rating = excluded.rating,
			comment = excluded.comment, created_at = excluded.created_at`,
		reviewerID, reviewedID, input.Rating, input.Comment, now())
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not save review")
		return
	}

	profile, _ := buildProfile(reviewedID, false)
	sendJSON(w, http.StatusCreated, profile)
}
