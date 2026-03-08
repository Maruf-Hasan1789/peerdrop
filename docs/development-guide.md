
## 🧑‍💻 Local Development Setup

If you want to run PeerDrop locally for development or contribute to the project.

### Prerequisites
Make sure you have installed:
- Go 1.24+
- Node.js 18+
- npm 9+
- Git

### Install Wails (if not installed)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Verify installation:

```bash
wails doctor
```

### Run in development mode

```bash
git clone https://github.com/Maruf-Hasan1789/peerdrop.git
cd peerdrop
cd frontend
npm install
cd ..
wails dev
```

This will start the backend and frontend in development mode with hot reload.

---



# 🤝 Contributing

Contributions are welcome! If you'd like to improve **PeerDrop**, follow these steps.

### 1️⃣ Fork the Repository

Click the **Fork** button on the repository page.

---

### 2️⃣ Clone Your Fork

```bash
git clone https://github.com/YOUR_USERNAME/peerdrop.git
cd peerdrop
```

---

### 3️⃣ Create a Branch

```bash
git checkout -b feature/your-feature-name
```

---

### 4️⃣ Make Changes

Implement your feature, fix, or improvement.

Please ensure:

* The project builds successfully
* Changes are focused and minimal
* No unrelated files are modified

---

### 5️⃣ Commit Changes

```bash
git commit -m "Peerdrop: short description of change"
```

---

### 6️⃣ Push to GitHub

```bash
git push origin feature/your-feature-name
```

---

### 7️⃣ Open a Pull Request

Go to the main repository and open a **Pull Request** describing your changes.

---