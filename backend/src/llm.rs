//! LLM integration using OpenRouter (OpenAI-compatible API)

use async_openai::{
    config::OpenAIConfig,
    types::{
        ChatCompletionRequestSystemMessageArgs, ChatCompletionRequestUserMessageArgs,
        CreateChatCompletionRequestArgs,
    },
    Client,
};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum LlmError {
    #[error("OpenAI API error: {0}")]
    ApiError(#[from] async_openai::error::OpenAIError),
    #[error("No response from model")]
    NoResponse,
    #[error("Missing API key: Set OPENROUTER_API_KEY environment variable")]
    MissingApiKey,
}

fn get_client() -> Result<Client<OpenAIConfig>, LlmError> {
    let api_key = std::env::var("OPENROUTER_API_KEY").map_err(|_| LlmError::MissingApiKey)?;

    let config = OpenAIConfig::new()
        .with_api_key(api_key)
        .with_api_base("https://openrouter.ai/api/v1");

    Ok(Client::with_config(config))
}

/// Optimize a LaTeX resume for a specific job description
pub async fn optimize_resume(
    resume_latex: &str,
    job_description: &str,
) -> Result<(String, String), LlmError> {
    let client = get_client()?;

    let system_prompt = r#"You are an expert resume optimizer. Your task is to modify a LaTeX resume to better match a job description while:
1. Keeping all LaTeX syntax valid and compilable
2. Preserving the original structure and formatting
3. Tailoring bullet points to highlight relevant experience
4. Adding relevant keywords from the job description naturally
5. Quantifying achievements where possible
6. Ensuring ATS compatibility (no tables, simple formatting)

IMPORTANT: Return ONLY valid LaTeX code. Do not include markdown code fences or explanations in the LaTeX output."#;

    let user_prompt = format!(
        r#"Please optimize this resume for the following job:

## Job Description:
{}

## Current Resume (LaTeX):
{}

Return the optimized LaTeX resume. After the LaTeX, add a separator "---CHANGES---" and briefly list the key changes you made."#,
        job_description, resume_latex
    );

    let request = CreateChatCompletionRequestArgs::default()
        .model("anthropic/claude-sonnet-4")
        .messages([
            ChatCompletionRequestSystemMessageArgs::default()
                .content(system_prompt)
                .build()?
                .into(),
            ChatCompletionRequestUserMessageArgs::default()
                .content(user_prompt)
                .build()?
                .into(),
        ])
        .max_tokens(4096u32)
        .build()?;

    let response = client.chat().create(request).await?;

    let content = response
        .choices
        .first()
        .and_then(|c| c.message.content.clone())
        .ok_or(LlmError::NoResponse)?;

    // Parse the response to separate LaTeX and changes summary
    let parts: Vec<&str> = content.split("---CHANGES---").collect();
    let optimized_latex = parts[0].trim().to_string();
    let changes_summary = parts.get(1).map(|s| s.trim()).unwrap_or("").to_string();

    Ok((optimized_latex, changes_summary))
}

/// Generate a cover letter based on resume and job description
pub async fn generate_cover_letter(
    resume_latex: &str,
    job_description: &str,
) -> Result<String, LlmError> {
    let client = get_client()?;

    let system_prompt = r#"You are an expert cover letter writer. Create a compelling, personalized cover letter that:
1. Highlights relevant experience from the resume
2. Shows genuine interest in the specific role and company
3. Connects the candidate's skills to job requirements
4. Uses a professional but engaging tone
5. Is concise (3-4 paragraphs, under 400 words)
6. Avoids generic phrases and clichés

Return only the cover letter text, ready to be used."#;

    let user_prompt = format!(
        r#"Write a cover letter for this job application:

## Job Description:
{}

## Candidate's Resume (LaTeX, extract relevant info):
{}"#,
        job_description, resume_latex
    );

    let request = CreateChatCompletionRequestArgs::default()
        .model("anthropic/claude-sonnet-4")
        .messages([
            ChatCompletionRequestSystemMessageArgs::default()
                .content(system_prompt)
                .build()?
                .into(),
            ChatCompletionRequestUserMessageArgs::default()
                .content(user_prompt)
                .build()?
                .into(),
        ])
        .max_tokens(1024u32)
        .build()?;

    let response = client.chat().create(request).await?;

    response
        .choices
        .first()
        .and_then(|c| c.message.content.clone())
        .ok_or(LlmError::NoResponse)
}
