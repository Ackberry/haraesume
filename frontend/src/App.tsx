import { useState, useCallback, useEffect } from 'react'
import './index.css'

interface ApiError {
  error: string
}

interface ResumeStatus {
  has_resume: boolean
}

interface CoverLetterApiResponse {
  cover_letter: string
  cover_letter_latex?: string
}

interface ApplicationPackageResponse {
  company_name: string
  folder_path: string
  resume_pdf_path?: string
  cover_letter_latex: string
  cover_letter_pdf_path?: string
  optimized_latex: string
  changes_summary: string
  pdf_warnings?: string[]
  tex_files_deleted: boolean
}

type Step = 'upload' | 'job' | 'optimize' | 'result'

const STEPS: Step[] = ['upload', 'job', 'optimize', 'result']

const STEP_LABELS: Record<Step, string> = {
  upload: 'Upload',
  job: 'Job',
  optimize: 'Optimize',
  result: 'Result',
}

const isTexFile = (file: File): boolean => file.name.toLowerCase().endsWith('.tex')

const UploadIcon = () => (
  <svg width="34" height="34" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true">
    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12" />
  </svg>
)

const FileIcon = () => (
  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <line x1="16" y1="13" x2="8" y2="13" />
    <line x1="16" y1="17" x2="8" y2="17" />
  </svg>
)

const CheckIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.3" aria-hidden="true">
    <polyline points="20 6 9 17 4 12" />
  </svg>
)

const SparkleIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
    <path d="M12 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L12 2Z" />
  </svg>
)

const DownloadIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" />
  </svg>
)

function App() {
  const [step, setStep] = useState<Step>('upload')
  const [resumeFile, setResumeFile] = useState<File | null>(null)
  const [resumeContent, setResumeContent] = useState<string>('')
  const [jobDescription, setJobDescription] = useState<string>('')
  const [optimizedLatex, setOptimizedLatex] = useState<string>('')
  const [changesSummary, setChangesSummary] = useState<string>('')
  const [coverLetter, setCoverLetter] = useState<string>('')
  const [loading, setLoading] = useState<boolean>(false)
  const [error, setError] = useState<string>('')
  const [dragOver, setDragOver] = useState<boolean>(false)
  const [hasSavedResume, setHasSavedResume] = useState<boolean>(false)
  const [savedPackage, setSavedPackage] = useState<ApplicationPackageResponse | null>(null)
  const [savingPackage, setSavingPackage] = useState<boolean>(false)

  const currentStepIndex = STEPS.indexOf(step)

  useEffect(() => {
    const checkResumeStatus = async () => {
      try {
        const res = await fetch('/api/resume-status')
        if (!res.ok) return

        const data: ResumeStatus = await res.json()
        if (data.has_resume) {
          setHasSavedResume(true)
          setStep('job')
        }
      } catch {
        // Keep upload as fallback.
      }
    }

    void checkResumeStatus()
  }, [])

  const handleFileSelect = useCallback(async (file: File) => {
    if (!isTexFile(file)) {
      setError('Please upload a .tex file only')
      return
    }

    setResumeFile(file)
    setError('')

    const content = await file.text()
    setResumeContent(content)

    setLoading(true)
    try {
      const formData = new FormData()
      formData.append('resume', file)

      const res = await fetch('/api/upload-resume', {
        method: 'POST',
        body: formData,
      })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      setHasSavedResume(true)
      setStep('job')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Upload failed')
    } finally {
      setLoading(false)
    }
  }, [])

  const handleDrop = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragOver(false)

    const file = e.dataTransfer.files[0]
    if (file && isTexFile(file)) {
      handleFileSelect(file)
    } else {
      setError('Please upload a .tex file')
    }
  }, [handleFileSelect])

  const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      handleFileSelect(file)
    }
  }, [handleFileSelect])

  const openFilePicker = useCallback(() => {
    document.getElementById('file-input')?.click()
  }, [])

  const handleUploadKeydown = useCallback((e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      openFilePicker()
    }
  }, [openFilePicker])

  const handleJobSubmit = async () => {
    if (!jobDescription.trim()) {
      setError('Please enter a job description')
      return
    }

    setLoading(true)
    setError('')
    setSavedPackage(null)

    try {
      const res = await fetch('/api/job-description', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ job_description: jobDescription }),
      })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      setStep('optimize')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save job description')
    } finally {
      setLoading(false)
    }
  }

  const handleOptimize = async () => {
    setLoading(true)
    setError('')
    setSavedPackage(null)

    try {
      const res = await fetch('/api/optimize', { method: 'POST' })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      const data = await res.json()
      setOptimizedLatex(data.optimized_latex)
      setChangesSummary(data.changes_summary)
      setStep('result')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Optimization failed')
    } finally {
      setLoading(false)
    }
  }

  const handleGenerateCoverLetter = async () => {
    setLoading(true)
    setError('')

    try {
      const res = await fetch('/api/cover-letter', { method: 'POST' })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      const data: CoverLetterApiResponse = await res.json()
      setCoverLetter(data.cover_letter_latex ?? data.cover_letter)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Cover letter generation failed')
    } finally {
      setLoading(false)
    }
  }

  const handleDownloadCoverLetterPdf = async () => {
    setLoading(true)
    setError('')

    try {
      const res = await fetch('/api/generate-cover-letter-pdf', { method: 'POST' })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      const data = await res.json()

      const byteCharacters = atob(data.pdf_base64)
      const byteNumbers = new Array(byteCharacters.length)
      for (let i = 0; i < byteCharacters.length; i += 1) {
        byteNumbers[i] = byteCharacters.charCodeAt(i)
      }

      const byteArray = new Uint8Array(byteNumbers)
      const blob = new Blob([byteArray], { type: 'application/pdf' })
      const url = URL.createObjectURL(blob)

      const a = document.createElement('a')
      a.href = url
      a.download = data.filename
      a.click()
      URL.revokeObjectURL(url)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Cover letter PDF generation failed')
    } finally {
      setLoading(false)
    }
  }

  const handleDownloadPdf = async () => {
    setLoading(true)
    setError('')

    try {
      const res = await fetch('/api/generate-pdf', { method: 'POST' })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      const data = await res.json()

      const byteCharacters = atob(data.pdf_base64)
      const byteNumbers = new Array(byteCharacters.length)
      for (let i = 0; i < byteCharacters.length; i += 1) {
        byteNumbers[i] = byteCharacters.charCodeAt(i)
      }

      const byteArray = new Uint8Array(byteNumbers)
      const blob = new Blob([byteArray], { type: 'application/pdf' })
      const url = URL.createObjectURL(blob)

      const a = document.createElement('a')
      a.href = url
      a.download = data.filename
      a.click()
      URL.revokeObjectURL(url)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'PDF generation failed')
    } finally {
      setLoading(false)
    }
  }

  const handleGenerateAndSavePackage = async () => {
    setLoading(true)
    setSavingPackage(true)
    setError('')

    try {
      const res = await fetch('/api/generate-application-package', { method: 'POST' })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      const data: ApplicationPackageResponse = await res.json()
      setSavedPackage(data)
      setOptimizedLatex(data.optimized_latex)
      setChangesSummary(data.changes_summary)
      setCoverLetter(data.cover_letter_latex)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Combined generation failed')
    } finally {
      setLoading(false)
      setSavingPackage(false)
    }
  }

  const handleReset = () => {
    setStep(hasSavedResume ? 'job' : 'upload')
    setResumeFile(null)
    setResumeContent('')
    setJobDescription('')
    setOptimizedLatex('')
    setChangesSummary('')
    setCoverLetter('')
    setSavedPackage(null)
    setSavingPackage(false)
    setError('')
  }

  return (
    <div className="app">
      <header className="site-header">
        <div className="container header-inner">
          <div>
            <p className="label">Resume Optimizer</p>
            <h1>Targeted Resume Tailoring</h1>
          </div>
          <ol className="stepper" aria-label="Progress">
            {STEPS.map((phase, index) => {
              const isComplete = currentStepIndex > index
              const isActive = currentStepIndex === index
              const itemClass = `step-item${isComplete ? ' complete' : ''}${isActive ? ' active' : ''}`

              return (
                <li key={phase} className={itemClass}>
                  <span className="step-index">{isComplete ? <CheckIcon /> : index + 1}</span>
                  <span className="step-text">{STEP_LABELS[phase]}</span>
                </li>
              )
            })}
          </ol>
        </div>
      </header>

      <main className="container main">
        {error && (
          <div className="alert" role="alert">
            {error}
          </div>
        )}

        {step === 'upload' && (
          <section className="card">
            <div className="section-head">
              <h2>Upload Resume</h2>
              <p>Upload a `.tex` file to start tailoring your resume.</p>
            </div>

            <div
              className={`dropzone${dragOver ? ' dragover' : ''}${resumeFile ? ' file-ready' : ''}`}
              onDragOver={(e) => {
                e.preventDefault()
                setDragOver(true)
              }}
              onDragLeave={() => setDragOver(false)}
              onDrop={handleDrop}
              onClick={openFilePicker}
              onKeyDown={handleUploadKeydown}
              tabIndex={0}
              role="button"
              aria-label="Upload LaTeX resume"
            >
              <input
                id="file-input"
                type="file"
                accept=".tex"
                className="sr-only"
                onChange={handleFileInput}
              />

              {loading ? (
                <div className="dropzone-state">
                  <div className="loader" />
                  <p>Uploading...</p>
                </div>
              ) : resumeFile ? (
                <div className="dropzone-state">
                  <FileIcon />
                  <p className="state-main">{resumeFile.name}</p>
                  <p className="state-sub">{(resumeFile.size / 1024).toFixed(1)} KB</p>
                </div>
              ) : (
                <div className="dropzone-state">
                  <UploadIcon />
                  <p className="state-main">Drop your file here</p>
                  <p className="state-sub">or click to browse</p>
                </div>
              )}
            </div>

            {resumeContent && (
              <div className="block">
                <h3>Preview</h3>
                <div className="code">{resumeContent.slice(0, 2000)}{resumeContent.length > 2000 ? '...' : ''}</div>
              </div>
            )}
          </section>
        )}

        {step === 'job' && (
          <section className="card">
            <div className="section-head">
              <h2>Job Description</h2>
              <p>Paste the full description to align your resume and cover letter.</p>
            </div>

            {hasSavedResume && (
              <div className="note">Using your saved base resume. Replace it if needed.</div>
            )}

            <textarea
              className="textarea"
              placeholder="Paste the full job description here..."
              value={jobDescription}
              onChange={(e) => setJobDescription(e.target.value)}
            />

            <div className="actions">
              <button className="btn btn-secondary" onClick={() => setStep('upload')}>
                {hasSavedResume ? 'Replace Resume' : 'Back'}
              </button>
              <button className="btn btn-primary" onClick={handleJobSubmit} disabled={loading}>
                {loading ? <span className="inline-loader"><span className="loader" />Saving</span> : 'Continue'}
              </button>
            </div>
          </section>
        )}

        {step === 'optimize' && (
          <section className="card card-center">
            <div className="icon-pill" aria-hidden="true">
              <SparkleIcon />
            </div>
            <h2>Ready to Optimize</h2>
            <p className="muted centered-text">Run the optimization to produce an ATS-focused version for this role.</p>

            <ul className="checklist">
              <li><CheckIcon /> Match responsibilities to your strongest impact points</li>
              <li><CheckIcon /> Introduce relevant keywords naturally</li>
              <li><CheckIcon /> Keep formatting ATS-safe</li>
              <li><CheckIcon /> Tighten phrasing for clarity and impact</li>
            </ul>

            <div className="actions center-actions">
              <button className="btn btn-secondary" onClick={() => setStep('job')}>Back</button>
              <button className="btn btn-primary" onClick={handleOptimize} disabled={loading}>
                {loading ? <span className="inline-loader"><span className="loader" />Optimizing</span> : 'Optimize Resume'}
              </button>
            </div>
          </section>
        )}

        {step === 'result' && (
          <section className="card">
            <div className="result-head">
              <div>
                <h2>Optimized Result</h2>
                <p className="muted">Review updates and export your final files.</p>
              </div>
              <button className="btn btn-secondary" onClick={handleReset}>Start Over</button>
            </div>

            {changesSummary && (
              <div className="block">
                <h3>Changes</h3>
                <p className="multiline muted">{changesSummary}</p>
              </div>
            )}

            <div className="block">
              <h3>Optimized LaTeX</h3>
              <div className="code code-large">{optimizedLatex}</div>
            </div>

            <div className="actions wrap-actions">
              <button className="btn btn-primary" onClick={handleDownloadPdf} disabled={loading}>
                {loading ? <span className="inline-loader"><span className="loader" />Building PDF</span> : <><DownloadIcon />Download PDF</>}
              </button>

              <button className="btn btn-primary" onClick={handleGenerateAndSavePackage} disabled={loading}>
                {savingPackage
                  ? <span className="inline-loader"><span className="loader" />Generating & Saving</span>
                  : <><SparkleIcon />Generate + Save Resume & Cover Letter</>}
              </button>

              <button
                className="btn btn-secondary"
                onClick={handleGenerateCoverLetter}
                disabled={loading || Boolean(coverLetter)}
              >
                {coverLetter ? <><CheckIcon />Cover Letter Generated</> : <><SparkleIcon />Generate Cover Letter</>}
              </button>

              {coverLetter && (
                <button className="btn btn-primary" onClick={handleDownloadCoverLetterPdf} disabled={loading}>
                  {loading ? <span className="inline-loader"><span className="loader" />Building PDF</span> : <><DownloadIcon />Download Cover Letter PDF</>}
                </button>
              )}
            </div>

            {savedPackage && (
              <div className="block">
                <h3>Saved Application Package</h3>
                <div className="code">
                  {[
                    `Company: ${savedPackage.company_name}`,
                    `Folder: ${savedPackage.folder_path}`,
                    savedPackage.tex_files_deleted ? 'Temporary .tex files: deleted' : 'Temporary .tex files: not fully deleted',
                    savedPackage.resume_pdf_path ? `Resume (.pdf): ${savedPackage.resume_pdf_path}` : '',
                    savedPackage.cover_letter_pdf_path ? `Cover Letter (.pdf): ${savedPackage.cover_letter_pdf_path}` : '',
                  ].filter(Boolean).join('\n')}
                </div>
                {savedPackage.pdf_warnings && savedPackage.pdf_warnings.length > 0 && (
                  <p className="multiline muted">{savedPackage.pdf_warnings.join('\n')}</p>
                )}
              </div>
            )}

            {coverLetter && (
              <div className="block">
                <h3>Cover Letter LaTeX</h3>
                <div className="code code-large">{coverLetter}</div>
                <button
                  className="btn btn-secondary"
                  onClick={() => {
                    navigator.clipboard.writeText(coverLetter)
                  }}
                >
                  Copy LaTeX to Clipboard
                </button>
              </div>
            )}
          </section>
        )}
      </main>

      <footer className="site-footer">
        <div className="container footer-inner">Built for focused, minimal resume workflow.</div>
      </footer>
    </div>
  )
}

export default App
