package main

// OBJECTIVE: simple cart-and-checkout flow for purchasing books
//
// The story: add books to your cart -> view cart -> checkout.
// At checkout the books become "sold" and an order is saved.
// (Payment is pretend here. A real site would call a payment service like Razorpay/Stripe.)

import (
	"net/http"
)

// POST /cart/{bookId}  (login needed)
func addToCartHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	bookID, err := getIDFromURL(r, "bookId")
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
		sendError(w, http.StatusBadRequest, "you can't buy your own book")
		return
	}
	if !book.ForSale {
		sendError(w, http.StatusBadRequest, "this book is not for sale")
		return
	}
	if book.Status != "available" {
		sendError(w, http.StatusConflict, "this book is not available anymore")
		return
	}

	// Already in the cart?
	var count int
	db.QueryRow("SELECT COUNT(*) FROM cart_items WHERE user_id = ? AND book_id = ?", userID, bookID).Scan(&count)
	if count > 0 {
		sendError(w, http.StatusConflict, "book is already in your cart")
		return
	}

	_, err = db.Exec("INSERT INTO cart_items (user_id, book_id) VALUES (?, ?)", userID, bookID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not add to cart")
		return
	}
	sendJSON(w, http.StatusCreated, map[string]string{"message": "added to cart"})
}

// DELETE /cart/{bookId}
func removeFromCartHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}
	bookID, err := getIDFromURL(r, "bookId")
	if err != nil {
		sendError(w, http.StatusBadRequest, "book id must be a number")
		return
	}

	db.Exec("DELETE FROM cart_items WHERE user_id = ? AND book_id = ?", userID, bookID)
	sendJSON(w, http.StatusOK, map[string]string{"message": "removed from cart"})
}

// GET /cart  -> books in my cart + total price
func viewCartHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	rows, err := db.Query("SELECT "+bookColumns+` FROM cart_items c
		JOIN books b ON b.id = c.book_id WHERE c.user_id = ?`, userID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not load cart")
		return
	}
	items := readBooks(rows)

	total := 0.0
	for _, b := range items {
		total += b.Price
	}
	sendJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items), "total": total})
}

// POST /checkout  -> buy everything in my cart
func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// A transaction: if ANY step fails, we undo everything (Rollback).
	// Nobody should be charged for half an order.
	tx, err := db.Begin()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Read the cart (inside the transaction)
	rows, err := tx.Query("SELECT "+bookColumns+` FROM cart_items c
		JOIN books b ON b.id = c.book_id WHERE c.user_id = ?`, userID)
	if err != nil {
		tx.Rollback()
		sendError(w, http.StatusInternalServerError, "could not read cart")
		return
	}
	items := readBooks(rows) // this also closes the rows

	if len(items) == 0 {
		tx.Rollback()
		sendError(w, http.StatusBadRequest, "your cart is empty")
		return
	}

	// Make sure every book can still be bought
	total := 0.0
	for _, b := range items {
		if !b.ForSale || b.Status != "available" {
			tx.Rollback()
			sendError(w, http.StatusConflict, "'"+b.Title+"' is no longer available, please remove it from your cart")
			return
		}
		total += b.Price
	}

	// Create the order
	result, err := tx.Exec("INSERT INTO orders (buyer_id, total, created_at) VALUES (?, ?, ?)", userID, total, now())
	if err != nil {
		tx.Rollback()
		sendError(w, http.StatusInternalServerError, "could not create order")
		return
	}
	orderID, _ := result.LastInsertId()

	// Save each book in the order and mark it sold
	for _, b := range items {
		if _, err := tx.Exec("INSERT INTO order_items (order_id, book_id, seller_id, price) VALUES (?, ?, ?, ?)",
			orderID, b.ID, b.OwnerID, b.Price); err != nil {
			tx.Rollback()
			sendError(w, http.StatusInternalServerError, "could not save order item")
			return
		}
		if _, err := tx.Exec("UPDATE books SET status = 'sold' WHERE id = ?", b.ID); err != nil {
			tx.Rollback()
			sendError(w, http.StatusInternalServerError, "could not update book")
			return
		}
		// A sold book must disappear from EVERYONE's cart
		if _, err := tx.Exec("DELETE FROM cart_items WHERE book_id = ?", b.ID); err != nil {
			tx.Rollback()
			sendError(w, http.StatusInternalServerError, "could not clear cart")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		sendError(w, http.StatusInternalServerError, "could not complete checkout")
		return
	}

	sendJSON(w, http.StatusCreated, map[string]any{
		"message":  "order placed!",
		"order_id": orderID,
		"total":    total,
	})
}

// GET /orders  -> my past purchases
func myOrdersHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getLoggedInUserID(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Step 1: load the orders
	rows, err := db.Query("SELECT id, total, created_at FROM orders WHERE buyer_id = ? ORDER BY id DESC", userID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "could not load orders")
		return
	}
	orders := []Order{}
	for rows.Next() {
		var o Order
		rows.Scan(&o.ID, &o.Total, &o.CreatedAt)
		o.Items = []OrderItem{}
		orders = append(orders, o)
	}
	rows.Close() // finish this loop BEFORE running more queries

	// Step 2: load the books inside each order
	for i := range orders {
		itemRows, err := db.Query(`SELECT oi.book_id, b.title, oi.price, oi.seller_id
			FROM order_items oi JOIN books b ON b.id = oi.book_id
			WHERE oi.order_id = ?`, orders[i].ID)
		if err != nil {
			continue
		}
		for itemRows.Next() {
			var item OrderItem
			itemRows.Scan(&item.BookID, &item.Title, &item.Price, &item.SellerID)
			orders[i].Items = append(orders[i].Items, item)
		}
		itemRows.Close()
	}

	sendJSON(w, http.StatusOK, orders)
}
