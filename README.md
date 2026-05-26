## How to run the project
1. Clone the repository to your local machine.
2. Navigate to the project directory.
3. use command `docker-compose up -d --build` to build and run the project in detached mode.
4. The application will be accessible at `http://localhost:8081`.
5. Test the application by sending a GET request to `http://localhost:8081/health` using a tool like Postman or your web browser. You should receive a response with the response 
`{
  "message": "Docker, Go, and Config are working perfectly! 🚀",
  "port": "8080",
  "status": "UP"
}`
