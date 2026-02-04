//! LaTeX to PDF compilation using TeX Live (pdflatex)

use std::process::Stdio;
use thiserror::Error;
use tokio::process::Command;
use uuid::Uuid;

#[derive(Error, Debug)]
pub enum LatexError {
    #[error("Failed to create temp directory: {0}")]
    TempDirError(#[from] std::io::Error),
    #[error("pdflatex compilation failed: {0}")]
    CompilationFailed(String),
    #[error("pdflatex not found. Please install TeX Live.")]
    PdflatexNotFound,
    #[error("Failed to read PDF output")]
    PdfReadError,
}

/// Compile LaTeX source to PDF using pdflatex
pub async fn compile_to_pdf(latex_source: &str) -> Result<Vec<u8>, LatexError> {
    // Create a unique temp directory for this compilation
    let temp_dir = std::env::temp_dir().join(format!("resume_{}", Uuid::new_v4()));
    tokio::fs::create_dir_all(&temp_dir).await?;

    let tex_path = temp_dir.join("resume.tex");
    let pdf_path = temp_dir.join("resume.pdf");

    // Write the LaTeX source
    tokio::fs::write(&tex_path, latex_source).await?;

    // Run pdflatex (twice for references, though resumes usually don't need it)
    for _ in 0..2 {
        let output = Command::new("pdflatex")
            .arg("-interaction=nonstopmode")
            .arg("-halt-on-error")
            .arg("-output-directory")
            .arg(&temp_dir)
            .arg(&tex_path)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .output()
            .await
            .map_err(|e| {
                if e.kind() == std::io::ErrorKind::NotFound {
                    LatexError::PdflatexNotFound
                } else {
                    LatexError::TempDirError(e)
                }
            })?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stdout);
            // pdflatex outputs errors to stdout, not stderr
            
            // Clean up temp files
            let _ = tokio::fs::remove_dir_all(&temp_dir).await;
            
            return Err(LatexError::CompilationFailed(
                extract_error_message(&stderr)
            ));
        }
    }

    // Read the generated PDF
    let pdf_bytes = tokio::fs::read(&pdf_path).await.map_err(|_| LatexError::PdfReadError)?;

    // Clean up temp files
    let _ = tokio::fs::remove_dir_all(&temp_dir).await;

    Ok(pdf_bytes)
}

/// Extract meaningful error message from pdflatex output
fn extract_error_message(log: &str) -> String {
    // Look for lines starting with "!" which indicate errors
    let errors: Vec<&str> = log
        .lines()
        .filter(|line| line.starts_with('!') || line.contains("Error:"))
        .take(5)
        .collect();

    if errors.is_empty() {
        "LaTeX compilation failed. Check your document syntax.".to_string()
    } else {
        errors.join("\n")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_simple_latex_compilation() {
        let simple_latex = r#"\documentclass{article}
\begin{document}
Hello, World!
\end{document}"#;

        let result = compile_to_pdf(simple_latex).await;
        
        // This test will only pass if pdflatex is installed
        match result {
            Ok(pdf) => {
                assert!(!pdf.is_empty());
                assert!(pdf.starts_with(b"%PDF")); // PDF magic bytes
            }
            Err(LatexError::PdflatexNotFound) => {
                println!("Skipping test: pdflatex not installed");
            }
            Err(e) => panic!("Unexpected error: {}", e),
        }
    }
}
