# Lab-5-Server-Side-Rendering
Servidor HTTP escrito en Go desde cero (sin usar net/http) que implementa un Series Tracker conectado a una base de datos SQLite.
Challenges: 

Estilos y CSS — La página tiene estilos propios con tabla formateada y botones.

Barra de progreso — Cada serie muestra una barra <progress> con los episodios vistos vs el total.

Marcar serie completa — Si el episodio actual es igual al total, se muestra el texto "Completa" en verde.

Botón -1 — Además del botón +1, hay un botón -1 para decrementar el episodio actual.

Función para eliminar serie — Botón de eliminar que usa el método HTTP DELETE.

Validación en servidor — El servidor valida que los campos no estén vacíos y que los números sean válidos antes de insertar.

Screenshot
<img width="1865" height="823" alt="image" src="https://github.com/user-attachments/assets/f58f070d-9647-40ae-b974-2f8c7b9d1636" />


Comando para correr el servidor: 
go run main.go
Luego abre http://localhost:8080 en tu navegador.
