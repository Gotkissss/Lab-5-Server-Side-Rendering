package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
	"net/url"
)

// ─────────────────────────────────────────────
// HELPERS — construir respuestas HTTP
// ─────────────────────────────────────────────

// Respuesta HTML normal (200 OK)
func respondHTML(conn net.Conn, html string) {
	response := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		html
	conn.Write([]byte(response))
}

// Redireccion (303 See Other) — se usa después de un POST para evitar reenviar el formulario
func respondRedirect(conn net.Conn, location string) {
	response := "HTTP/1.1 303 See Other\r\n" +
		"Location: " + location + "\r\n" +
		"\r\n"
	conn.Write([]byte(response))
}

// Respuesta de texto plano — la usamos para el endpoint /update que llama fetch()
func respondText(conn net.Conn, text string) {
	response := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		text
	conn.Write([]byte(response))
}

// Respuesta de error
func respondError(conn net.Conn, msg string) {
	response := "HTTP/1.1 500 Internal Server Error\r\n\r\n" + msg
	conn.Write([]byte(response))
}

// ─────────────────────────────────────────────
// HANDLERS — una función por ruta
// ─────────────────────────────────────────────

// GET / — muestra la tabla de series
func handleIndex(conn net.Conn, db *sql.DB) {
	rows, err := db.Query("SELECT id, name, current_episode, total_episodes FROM series")
	if err != nil {
		respondError(conn, "Error al consultar la base de datos")
		return
	}
	defer rows.Close()

	html := `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <title>Series Tracker</title>
  <style>
    body { font-family: Arial, sans-serif; padding: 20px; }
    table { border-collapse: collapse; width: 65%; }
    th, td { border: 1px solid #ccc; padding: 8px; text-align: left; }
    th { background-color: white; color: black; }
    .btn { padding: 3px 8px; cursor: pointer; font-size: 13px; border: 1px solid #aaa; background: #f0f0f0; border-radius: 3px; }
    .btn:hover { background: #e0e0e0; }
    .btn-del { color: #c00; }
    .complete { color: green; font-size: 12px; margin-left: 6px; }
    progress { width: 90px; height: 12px; vertical-align: middle; }
    a { color: #00c; }
  </style>
</head>
<body>
  <h2>Track de mis series actuales</h2>
  <table>
    <tr>
      <th>No.</th>
      <th>Nombre</th>
      <th>Progreso</th>
      <th>Episodio actual</th>
      <th>Total de episodios</th>
      <th>Acciones</th>
    </tr>`

	for rows.Next() {
		var id, current, total int
		var name string
		if err := rows.Scan(&id, &name, &current, &total); err != nil {
			continue
		}

		// Si está completa, mostramos un texto especial
		statusLabel := ""
		if current >= total {
			statusLabel = ` <span class="complete">✔ Completa</span>`
		}

		html += fmt.Sprintf(`
    <tr>
      <td>%d</td>
      <td>%s%s</td>
      <td><progress value="%d" max="%d"></progress> %d/%d</td>
      <td>%d</td>
      <td>%d</td>
      <td>
        <button class="btn" onclick="cambiarEpisodio(%d,  1)">+1</button>
        <button class="btn" onclick="cambiarEpisodio(%d, -1)">-1</button>
        <button class="btn btn-del" onclick="eliminarSerie(%d)">eliminar</button>
      </td>
    </tr>`, id, name, statusLabel, current, total, current, total, current, total, id, id, id)
	}

	html += `
  </table>
  <br>
  <a href="/create">+ Agregar nueva serie</a>

  <script>
    // Función genérica para +1 y -1
    async function cambiarEpisodio(id, delta) {
      await fetch('/update?id=' + id + '&delta=' + delta, { method: 'POST' });
      location.reload();
    }

    // Eliminar serie usando método DELETE
    async function eliminarSerie(id) {
      if (!confirm('¿Eliminar esta serie?')) return;
      await fetch('/delete?id=' + id, { method: 'DELETE' });
      location.reload();
    }
  </script>
</body>
</html>`

	respondHTML(conn, html)
}

// GET /create — muestra el formulario para agregar una serie
func handleCreateForm(conn net.Conn) {
	html := `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <title>Agregar Serie</title>
  <style>
    body { font-family: Arial, sans-serif; padding: 20px; }
    label { display: block; margin-top: 10px; }
    input { padding: 5px; margin-top: 3px; width: 250px; }
    button { margin-top: 14px; padding: 6px 14px; cursor: pointer; }
    a { color: #00c; }
  </style>
</head>
<body>
  <h2>Agregar nueva serie</h2>
  <form method="POST" action="/create">
    <label>Nombre de la serie</label>
    <input type="text" name="series_name" placeholder="Ej: Attack on Titan" required>

    <label>Episodio actual</label>
    <input type="number" name="current_episode" min="1" value="1" required>

    <label>Total de episodios</label>
    <input type="number" name="total_episodes" min="1" required>

    <br>
    <button type="submit">Guardar</button>
  </form>
  <br>
  <a href="/">← Volver</a>
</body>
</html>`

	respondHTML(conn, html)
}

// POST /create — recibe los datos del formulario, inserta en DB y redirige
func handleCreatePost(conn net.Conn, db *sql.DB, body string) {
	// El body llega así: series_name=Steins%3BGate&current_episode=1&total_episodes=24
	// url.ParseQuery decodifica eso en un mapa clave→valor
	values, err := url.ParseQuery(body)
	if err != nil {
		respondError(conn, "Error al parsear el formulario")
		return
	}

	name := strings.TrimSpace(values.Get("series_name"))
	currentEpStr := values.Get("current_episode")
	totalEpsStr := values.Get("total_episodes")

	// Validación básica en el servidor
	if name == "" || currentEpStr == "" || totalEpsStr == "" {
		respondError(conn, "Todos los campos son obligatorios")
		return
	}

	currentEp, err1 := strconv.Atoi(currentEpStr)
	totalEps, err2 := strconv.Atoi(totalEpsStr)
	if err1 != nil || err2 != nil || currentEp < 1 || totalEps < 1 || currentEp > totalEps {
		respondError(conn, "Valores numéricos inválidos")
		return
	}

	_, err = db.Exec(
		"INSERT INTO series (name, current_episode, total_episodes) VALUES (?, ?, ?)",
		name, currentEp, totalEps,
	)
	if err != nil {
		respondError(conn, "Error al insertar en la base de datos")
		return
	}

	// Patrón POST/Redirect/GET: después de insertar, redirigimos al inicio
	// Así si el usuario recarga la página no vuelve a enviar el formulario
	respondRedirect(conn, "/")
}

// POST /update?id=X&delta=1 — incrementa o decrementa el episodio actual
func handleUpdate(conn net.Conn, db *sql.DB, query string) {
	params, _ := url.ParseQuery(query)
	id := params.Get("id")
	delta := params.Get("delta")

	if delta == "1" {
		db.Exec(`UPDATE series SET current_episode = current_episode + 1
		         WHERE id = ? AND current_episode < total_episodes`, id)
	} else if delta == "-1" {
		db.Exec(`UPDATE series SET current_episode = current_episode - 1
		         WHERE id = ? AND current_episode > 1`, id)
	}

	respondText(conn, "ok")
}

// DELETE /delete?id=X — elimina una serie
func handleDelete(conn net.Conn, db *sql.DB, query string) {
	params, _ := url.ParseQuery(query)
	id := params.Get("id")

	db.Exec("DELETE FROM series WHERE id = ?", id)
	respondText(conn, "ok")
}

// ─────────────────────────────────────────────
// ROUTER — lee el request y decide qué handler usar
// ─────────────────────────────────────────────

func handleClient(conn net.Conn, db *sql.DB) {
	defer conn.Close()

	// Usamos bufio.Reader para leer línea por línea de forma segura
	reader := bufio.NewReader(conn)

	// La primera línea del request HTTP es: "MÉTODO /ruta HTTP/1.1"
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	requestLine = strings.TrimSpace(requestLine)
	parts := strings.Split(requestLine, " ")
	if len(parts) < 2 {
		return
	}

	method := parts[0]   // "GET", "POST", "DELETE", etc.
	fullPath := parts[1] // "/create", "/update?id=3", etc.

	// Separar la ruta del query string: "/update?id=3" → ruta="/update", query="id=3"
	pathParts := strings.SplitN(fullPath, "?", 2)
	route := pathParts[0]
	query := ""
	if len(pathParts) > 1 {
		query = pathParts[1]
	}

	// Leer los headers para obtener Content-Length (necesario para el POST)
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // Línea vacía = fin de los headers, lo que sigue es el body
		}
		if strings.HasPrefix(line, "Content-Length:") {
			lenStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(lenStr)
		}
	}

	// Leer el body si existe (solo en POST)
	body := ""
	if contentLength > 0 {
		bodyBytes := make([]byte, contentLength)
		reader.Read(bodyBytes)
		body = string(bodyBytes)
	}

	// Routing: decidir qué función llamar según método + ruta
	switch {
	case method == "GET" && route == "/":
		handleIndex(conn, db)

	case method == "GET" && route == "/create":
		handleCreateForm(conn)

	case method == "POST" && route == "/create":
		handleCreatePost(conn, db, body)

	case method == "POST" && route == "/update":
		handleUpdate(conn, db, query)

	case method == "DELETE" && route == "/delete":
		handleDelete(conn, db, query)

	default:
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n<h1>404 - Página no encontrada</h1>"))
	}
}

// ─────────────────────────────────────────────
// MAIN — inicia la DB y el servidor TCP
// ─────────────────────────────────────────────

func main() {
	// Abrir (o crear) la base de datos SQLite
	db, err := sql.Open("sqlite", "file:series.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Crear la tabla si no existe todavía
	db.Exec(`CREATE TABLE IF NOT EXISTS series (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		name            TEXT NOT NULL,
		current_episode INTEGER NOT NULL,
		total_episodes  INTEGER NOT NULL
	)`)

	// Escuchar conexiones TCP en el puerto 8080
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}

	fmt.Println("Servidor corriendo en http://localhost:8080")

	// Bucle principal: aceptar y manejar conexiones
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		// go = goroutine, maneja cada cliente en paralelo sin bloquear el bucle
		go handleClient(conn, db)
	}
}