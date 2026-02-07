import { useState, useCallback } from 'react'
import './index.css'

// Types
interface ApiError {
  error: string
}

type Step = 'upload' | 'job' | 'optimize' | 'result'
const isTexFile = (file: File): boolean => file.name.toLowerCase().endsWith('.tex')

// Icons as simple SVG components
const UploadIcon = () => (
  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12" />
  </svg>
)

const FileIcon = () => (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <line x1="16" y1="13" x2="8" y2="13" />
    <line x1="16" y1="17" x2="8" y2="17" />
  </svg>
)

const CheckIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    <polyline points="20 6 9 17 4 12" />
  </svg>
)

const SparkleIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
    <path d="M12 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L12 2Z" />
  </svg>
)

const DownloadIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
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

  // Handle file upload
  const handleFileSelect = useCallback(async (file: File) => {
    if (!isTexFile(file)) {
      setError('Please upload a .tex file only')
      return
    }

    setResumeFile(file)
    setError('')

    // Read file content for preview
    const content = await file.text()
    setResumeContent(content)

    // Upload to backend
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

      setStep('job')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Upload failed')
    } finally {
      setLoading(false)
    }
  }, [])

  const handleDrop = useCallback((e: React.DragEvent) => {
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
    if (file) handleFileSelect(file)
  }, [handleFileSelect])

  // Submit job description
  const handleJobSubmit = async () => {
    if (!jobDescription.trim()) {
      setError('Please enter a job description')
      return
    }

    setLoading(true)
    setError('')

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

  // Optimize resume
  const handleOptimize = async () => {
    setLoading(true)
    setError('')

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

  // Generate cover letter
  const handleGenerateCoverLetter = async () => {
    setLoading(true)
    setError('')

    try {
      const res = await fetch('/api/cover-letter', { method: 'POST' })

      if (!res.ok) {
        const err: ApiError = await res.json()
        throw new Error(err.error)
      }

      const data = await res.json()
      setCoverLetter(data.cover_letter)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Cover letter generation failed')
    } finally {
      setLoading(false)
    }
  }

  // Download PDF
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

      // Convert base64 to blob and download
      const byteCharacters = atob(data.pdf_base64)
      const byteNumbers = new Array(byteCharacters.length)
      for (let i = 0; i < byteCharacters.length; i++) {
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

  // Reset to start
  const handleReset = () => {
    setStep('upload')
    setResumeFile(null)
    setResumeContent('')
    setJobDescription('')
    setOptimizedLatex('')
    setChangesSummary('')
    setCoverLetter('')
    setError('')
  }

  return (
    <div className="gradient-bg min-h-screen">
      {/* Header */}
      <header className="border-b border-[var(--border-color)] px-8 py-6">
        <div className="max-w-6xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl gradient-accent flex items-center justify-center">
              <SparkleIcon />
            </div>
            <h1 className="text-xl font-bold">Resume Optimizer</h1>
          </div>

          {/* Progress indicator */}
          <div className="flex items-center gap-2">
            {(['upload', 'job', 'optimize', 'result'] as Step[]).map((s, i) => (
              <div key={s} className="flex items-center">
                <div className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium transition-all
                  ${step === s ? 'gradient-accent text-white animate-pulse-glow' :
                    ['upload', 'job', 'optimize', 'result'].indexOf(step) > i ?
                      'bg-[var(--success)] text-white' : 'bg-[var(--bg-card)] text-[var(--text-muted)]'}`}>
                  {['upload', 'job', 'optimize', 'result'].indexOf(step) > i ? <CheckIcon /> : i + 1}
                </div>
                {i < 3 && <div className={`w-12 h-0.5 mx-1 transition-all
                  ${['upload', 'job', 'optimize', 'result'].indexOf(step) > i ? 'bg-[var(--success)]' : 'bg-[var(--border-color)]'}`} />}
              </div>
            ))}
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-4xl mx-auto px-8 py-12">
        {/* Error display */}
        {error && (
          <div className="badge badge-error mb-6 fade-in">
            ⚠️ {error}
          </div>
        )}

        {/* Step 1: Upload */}
        {step === 'upload' && (
          <div className="fade-in">
            <h2 className="text-3xl font-bold mb-2">Upload Your Resume</h2>
            <p className="text-[var(--text-secondary)] mb-8">
              Upload your LaTeX resume (.tex file) to get started
            </p>

            <div
              className={`upload-zone ${dragOver ? 'dragover' : ''} ${resumeFile ? 'has-file' : ''}`}
              onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
              onDragLeave={() => setDragOver(false)}
              onDrop={handleDrop}
              onClick={() => document.getElementById('file-input')?.click()}
            >
              <input
                id="file-input"
                type="file"
                accept=".tex"
                className="hidden"
                onChange={handleFileInput}
              />

              {loading ? (
                <div className="flex flex-col items-center gap-4">
                  <div className="loader" />
                  <p className="text-[var(--text-secondary)]">Uploading...</p>
                </div>
              ) : resumeFile ? (
                <div className="flex flex-col items-center gap-4">
                  <div className="text-[var(--success)]"><FileIcon /></div>
                  <p className="font-medium">{resumeFile.name}</p>
                  <p className="text-[var(--text-muted)] text-sm">
                    {(resumeFile.size / 1024).toFixed(1)} KB
                  </p>
                </div>
              ) : (
                <div className="flex flex-col items-center gap-4">
                  <div className="text-[var(--text-muted)]"><UploadIcon /></div>
                  <div>
                    <p className="font-medium">Drop your .tex file here</p>
                    <p className="text-[var(--text-muted)] text-sm mt-1">
                      or click to browse
                    </p>
                  </div>
                </div>
              )}
            </div>

            {resumeContent && (
              <div className="mt-8">
                <h3 className="text-lg font-semibold mb-3">Preview</h3>
                <div className="code-display max-h-64 overflow-y-auto">
                  {resumeContent.slice(0, 2000)}{resumeContent.length > 2000 ? '...' : ''}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Step 2: Job Description */}
        {step === 'job' && (
          <div className="fade-in">
            <h2 className="text-3xl font-bold mb-2">Paste Job Description</h2>
            <p className="text-[var(--text-secondary)] mb-8">
              Paste the job description to tailor your resume
            </p>

            <textarea
              className="input-field textarea"
              placeholder="Paste the full job description here..."
              value={jobDescription}
              onChange={(e) => setJobDescription(e.target.value)}
            />

            <div className="flex gap-4 mt-6">
              <button className="btn-secondary" onClick={() => setStep('upload')}>
                ← Back
              </button>
              <button
                className="btn-primary flex-1 flex items-center justify-center gap-2"
                onClick={handleJobSubmit}
                disabled={loading}
              >
                {loading ? <div className="loader" /> : <>Continue →</>}
              </button>
            </div>
          </div>
        )}

        {/* Step 3: Optimize */}
        {step === 'optimize' && (
          <div className="fade-in text-center py-12">
            <div className="w-20 h-20 rounded-2xl gradient-accent flex items-center justify-center mx-auto mb-6 animate-pulse-glow">
              <SparkleIcon />
            </div>
            <h2 className="text-3xl font-bold mb-4">Ready to Optimize</h2>
            <p className="text-[var(--text-secondary)] mb-8 max-w-md mx-auto">
              We'll analyze your resume against the job description and optimize it for ATS systems
            </p>

            <div className="glass-card p-6 max-w-md mx-auto mb-8 text-left">
              <h3 className="font-semibold mb-3">What we'll do:</h3>
              <ul className="space-y-2 text-[var(--text-secondary)]">
                <li className="flex items-center gap-2">
                  <CheckIcon /> Tailor bullet points to job requirements
                </li>
                <li className="flex items-center gap-2">
                  <CheckIcon /> Add relevant keywords naturally
                </li>
                <li className="flex items-center gap-2">
                  <CheckIcon /> Ensure ATS compatibility
                </li>
                <li className="flex items-center gap-2">
                  <CheckIcon /> Quantify achievements where possible
                </li>
              </ul>
            </div>

            <div className="flex gap-4 justify-center">
              <button className="btn-secondary" onClick={() => setStep('job')}>
                ← Back
              </button>
              <button
                className="btn-primary flex items-center gap-2"
                onClick={handleOptimize}
                disabled={loading}
              >
                {loading ? (
                  <>
                    <div className="loader" />
                    Optimizing...
                  </>
                ) : (
                  <>
                    <SparkleIcon /> Optimize Resume
                  </>
                )}
              </button>
            </div>
          </div>
        )}

        {/* Step 4: Results */}
        {step === 'result' && (
          <div className="fade-in">
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-3xl font-bold">Your Optimized Resume</h2>
                <p className="text-[var(--text-secondary)]">
                  Review the changes and download your new resume
                </p>
              </div>
              <button className="btn-secondary" onClick={handleReset}>
                Start Over
              </button>
            </div>

            {/* Changes summary */}
            {changesSummary && (
              <div className="glass-card p-6 mb-6">
                <h3 className="font-semibold mb-3 flex items-center gap-2">
                  <SparkleIcon /> Changes Made
                </h3>
                <p className="text-[var(--text-secondary)] whitespace-pre-wrap">
                  {changesSummary}
                </p>
              </div>
            )}

            {/* Optimized LaTeX */}
            <div className="mb-6">
              <h3 className="text-lg font-semibold mb-3">Optimized LaTeX</h3>
              <div className="code-display max-h-96 overflow-y-auto">
                {optimizedLatex}
              </div>
            </div>

            {/* Action buttons */}
            <div className="flex flex-wrap gap-4 mb-8">
              <button
                className="btn-primary flex items-center gap-2"
                onClick={handleDownloadPdf}
                disabled={loading}
              >
                {loading ? <div className="loader" /> : <DownloadIcon />}
                Download PDF
              </button>

              <button
                className="btn-secondary flex items-center gap-2"
                onClick={handleGenerateCoverLetter}
                disabled={loading || !!coverLetter}
              >
                {coverLetter ? <CheckIcon /> : <SparkleIcon />}
                {coverLetter ? 'Cover Letter Generated' : 'Generate Cover Letter'}
              </button>
            </div>

            {/* Cover letter */}
            {coverLetter && (
              <div className="glass-card p-6 fade-in">
                <h3 className="font-semibold mb-4 flex items-center gap-2">
                  📝 Your Cover Letter
                </h3>
                <div className="text-[var(--text-secondary)] whitespace-pre-wrap leading-relaxed">
                  {coverLetter}
                </div>
                <button
                  className="btn-secondary mt-4"
                  onClick={() => {
                    navigator.clipboard.writeText(coverLetter)
                  }}
                >
                  Copy to Clipboard
                </button>
              </div>
            )}
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-[var(--border-color)] px-8 py-6 mt-auto">
        <div className="max-w-6xl mx-auto text-center text-[var(--text-muted)] text-sm">
          Powered by AI • LaTeX + TeX Live • Made with ♥
        </div>
      </footer>
    </div>
  )
}

export default App
