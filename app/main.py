from fastapi import FastAPI, Depends
from fastapi.openapi.utils import get_openapi
from fastapi.security import OAuth2PasswordBearer

# Import routers
from sdlc.api.routes.users import router as users_router

# Define OAuth2 scheme
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="/auth/token")

tags = []  # Define any tags if needed

# Initialize FastAPI app with custom openapi URL
app = FastAPI(openapi_tags=tags, openapi_url="/openapi.json")

# Include routers with security dependencies
app.include_router(users_router, dependencies=[Depends(oauth2_scheme)])

# Set custom OpenAPI schema with security definitions
app.openapi_schema = get_openapi(
    title="SDLC API",
    version="1.0.0",
    routes=app.routes,
    components={"securitySchemes": {"BearerAuth": {"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}}},
    security=[{"BearerAuth": []}],
)
