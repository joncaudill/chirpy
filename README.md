# chirpy
Chirpy http server project

### Purpose
An http server project to learn the ins and outs of building api end points, authentication, authorizationa and message passing in go.

### Technical notes and setup
this project requires postgresql to store the data.

in the root of your user directory it will look for a .env file that contains the following strings

DB_URL="postgres://*yourpostgresusername*:*yourpostgrespassword*@*servername*:*port#*/chirpy?sslmode=disable"
PLATFORM="dev"
JWT_SECRET="*your secret string for encoding JWT tokens*"
POLKA_KEY="*whatever api key you want to use for pretending you have pro users here :)*"

The polka key was just to practice passing along api keys in an authorization header

you can generate long secure keys from the command line like this:

openssl rand -base64 64

To create the database tables, you can go to the sql/schema and use goose to run up the migration until all 5 are up like this:

goose postgres "postgres://*username*:*password*@*yourserver*:*port#*/*yourdatabase*" up

to create the executable:

go build

it will by default run a webserver on port 8080

### server endpoints
The endpoints for the server are:

- GET /api/healthz : see if the system is ready to run
- GET /admin/metrics : check the number of hits the app gets on the /app/ endpoint
- GET /api/chirps/" : gets all chirps.  Accepts url queries for author_id=*author's UUID* and sort=*asc or desc*
- POST /api/chirps" : post a chirp.  Checks for authentication tokens in the header for authorization.
- GET /api/chirps/{chirpID} : get a chirp given chirpID
- DELETE /api/chirps/{chirpID} : delete a chirp given chirpID.  Checks for auth to make sure you can only delete your own chirps
- POST /admin/reset" : reset all chirp, users, tokens
- POST /api/users" : create a user
- PUT /api/users" : update a users's email and password. uses auth to make sure you can only update your own information.
- POST /api/login" : login a user
- POST /api/refresh" : update the users JWTToken
- POST /api/revoke" : revoke a users refresh token
- POST /api/polka/webhooks" : handle payments from a fake payment company's webhooks .  Handles api key in env file to make sure the webhook is valid.


