use axum::{
    extract::{Multipart, State},
    http::StatusCode,
    response::Json,
    routing::{get, post},
    Router,
};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use tokio::sync::RwLock;
use tower_http::cors::{Any, CorsLayer};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

mod llm;
mod latex;

// Application state
#[derive(Default)]
pub struct AppState {
    // Store the current resume and job description in memory for simplicity
    pub current_resume: RwLock<Option<String>>,
    pub current_job_description: RwLock<Option<String>>,
}

// API Request/Response types
#[derive(Deserialize)]
pub struct JobDescriptionRequest {
    pub job_description: String,
}

#[derive(Serialize)]
pub struct OptimizeResponse {
    pub optimized_latex: String,
    pub changes_summary: String,
}

#[derive(Serialize)]
pub struct CoverLetterResponse {
    pub cover_letter: String,
}

#[derive(Serialize)]
pub struct PdfResponse {
    pub pdf_base64: String,
    pub filename: String,
}

#[derive(Serialize)]
pub struct ErrorResponse {
    pub error: String,
}

#[derive(Serialize)]
pub struct HealthResponse {
    pub status: String,
    pub version: String,
}

#[tokio::main]
async fn main() {
    // Initialize tracing
    tracing_subscriber::registry()
        .with(tracing_subscriber::EnvFilter::new(
            std::env::var("RUST_LOG").unwrap_or_else(|_| "info".into()),
        ))
        .with(tracing_subscriber::fmt::layer())
        .init();

    let state = Arc::new(AppState::default());

    // CORS configuration for frontend
    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    let app = Router::new()
        .route("/health", get(health_check))
        .route("/api/upload-resume", post(upload_resume))
        .route("/api/job-description", post(set_job_description))
        .route("/api/optimize", post(optimize_resume))
        .route("/api/cover-letter", post(generate_cover_letter))
        .route("/api/generate-pdf", post(generate_pdf))
        .layer(cors)
        .with_state(state);

    let listener = tokio::net::TcpListener::bind("0.0.0.0:3001").await.unwrap();
    tracing::info!("🚀 Server running on http://localhost:3001");
    axum::serve(listener, app).await.unwrap();
}

async fn health_check() -> Json<HealthResponse> {
    Json(HealthResponse {
        status: "healthy".to_string(),
        version: env!("CARGO_PKG_VERSION").to_string(),
    })
}

async fn upload_resume(
    State(state): State<Arc<AppState>>,
    mut multipart: Multipart,
) -> Result<Json<serde_json::Value>, (StatusCode, Json<ErrorResponse>)> {
    while let Some(field) = multipart.next_field().await.map_err(|e| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse {
                error: format!("Failed to read multipart: {}", e),
            }),
        )
    })? {
        if field.name() == Some("resume") {
            let data = field.bytes().await.map_err(|e| {
                (
                    StatusCode::BAD_REQUEST,
                    Json(ErrorResponse {
                        error: format!("Failed to read file: {}", e),
                    }),
                )
            })?;
            
            let content = String::from_utf8(data.to_vec()).map_err(|_| {
                (
                    StatusCode::BAD_REQUEST,
                    Json(ErrorResponse {
                        error: "File must be valid UTF-8 (LaTeX)".to_string(),
                    }),
                )
            })?;

            *state.current_resume.write().await = Some(content.clone());
            
            return Ok(Json(serde_json::json!({
                "success": true,
                "message": "Resume uploaded successfully",
                "length": content.len()
            })));
        }
    }

    Err((
        StatusCode::BAD_REQUEST,
        Json(ErrorResponse {
            error: "No resume file found in request".to_string(),
        }),
    ))
}

async fn set_job_description(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<JobDescriptionRequest>,
) -> Json<serde_json::Value> {
    *state.current_job_description.write().await = Some(payload.job_description);
    Json(serde_json::json!({
        "success": true,
        "message": "Job description saved"
    }))
}

async fn optimize_resume(
    State(state): State<Arc<AppState>>,
) -> Result<Json<OptimizeResponse>, (StatusCode, Json<ErrorResponse>)> {
    let resume = state.current_resume.read().await.clone().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse {
                error: "No resume uploaded. Please upload a resume first.".to_string(),
            }),
        )
    })?;

    let job_description = state.current_job_description.read().await.clone().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse {
                error: "No job description provided. Please set a job description first.".to_string(),
            }),
        )
    })?;

    let (optimized_latex, changes_summary) = llm::optimize_resume(&resume, &job_description)
        .await
        .map_err(|e| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse {
                    error: format!("LLM error: {}", e),
                }),
            )
        })?;

    // Store the optimized version
    *state.current_resume.write().await = Some(optimized_latex.clone());

    Ok(Json(OptimizeResponse {
        optimized_latex,
        changes_summary,
    }))
}

async fn generate_cover_letter(
    State(state): State<Arc<AppState>>,
) -> Result<Json<CoverLetterResponse>, (StatusCode, Json<ErrorResponse>)> {
    let resume = state.current_resume.read().await.clone().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse {
                error: "No resume uploaded".to_string(),
            }),
        )
    })?;

    let job_description = state.current_job_description.read().await.clone().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse {
                error: "No job description provided".to_string(),
            }),
        )
    })?;

    let cover_letter = llm::generate_cover_letter(&resume, &job_description)
        .await
        .map_err(|e| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse {
                    error: format!("LLM error: {}", e),
                }),
            )
        })?;

    Ok(Json(CoverLetterResponse { cover_letter }))
}

async fn generate_pdf(
    State(state): State<Arc<AppState>>,
) -> Result<Json<PdfResponse>, (StatusCode, Json<ErrorResponse>)> {
    let resume = state.current_resume.read().await.clone().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse {
                error: "No resume to convert".to_string(),
            }),
        )
    })?;

    let pdf_bytes = latex::compile_to_pdf(&resume).await.map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse {
                error: format!("PDF generation failed: {}", e),
            }),
        )
    })?;

    use base64::Engine;
    let pdf_base64 = base64::engine::general_purpose::STANDARD.encode(&pdf_bytes);

    Ok(Json(PdfResponse {
        pdf_base64,
        filename: "resume.pdf".to_string(),
    }))
}
