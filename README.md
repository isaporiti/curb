# 🔁 curb

**curb** is a small, composable Go package that provides retry logic with exponential backoff and jitter. It's designed for simplicity, testability, and real-world use cases like retrying API calls or transient network operations.

---

## ✨ Features

- Generic retry helper with `Retry[T]`
- Exponential backoff with jitter
- Configurable retry attempts (default: 5)
- Clean and minimal API

---

## 📦 Installation

```bash
go get github.com/isaporiti/curb


