import { useState, useCallback, useEffect, useRef } from 'react'
import {
  Alert,
  AlertIcon,
  Box,
  Button,
  Flex,
  Heading,
  HStack,
  Icon,
  List,
  ListIcon,
  ListItem,
  Spinner,
  Stack,
  Text,
  Textarea,
  type IconProps,
} from '@chakra-ui/react'

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

interface OptimizeApiResponse {
  optimized_latex: string
  changes_summary: string
}

interface ResumePdfApiResponse {
  pdf_base64: string
  filename: string
  company_name?: string
  folder_path?: string
  resume_pdf_path?: string
  pdf_warnings?: string[]
  tex_files_deleted?: boolean
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
  upload: 'upload',
  job: 'job',
  optimize: 'optimize',
  result: 'result',
}

const isTexFile = (file: File): boolean => file.name.toLowerCase().endsWith('.tex')

const UploadIcon = (props: IconProps) => (
  <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" {...props}>
    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12" />
  </Icon>
)

const FileIcon = (props: IconProps) => (
  <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" {...props}>
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
    <polyline points="14 2 14 8 20 8" />
    <line x1="16" y1="13" x2="8" y2="13" />
    <line x1="16" y1="17" x2="8" y2="17" />
  </Icon>
)

const CheckIcon = (props: IconProps) => (
  <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" {...props}>
    <polyline points="20 6 9 17 4 12" />
  </Icon>
)

const SparkleIcon = (props: IconProps) => (
  <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" {...props}>
    <path d="M12 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L12 2Z" />
  </Icon>
)

const DownloadIcon = (props: IconProps) => (
  <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" {...props}>
    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" />
  </Icon>
)

/* ── Shared styles ─────────────────────────────────────── */

const codeBlock = {
  as: 'pre' as const,
  p: 4,
  borderLeft: '2px solid',
  borderColor: 'ink.200',
  bg: 'ink.50',
  fontSize: 'xs',
  lineHeight: '1.6',
  maxH: '360px',
  overflow: 'auto',
  whiteSpace: 'pre-wrap' as const,
  wordBreak: 'break-word' as const,
  textAlign: 'left' as const,
}

/* ── Component ─────────────────────────────────────────── */

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

  const fileInputRef = useRef<HTMLInputElement>(null)
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
      setError('please upload a .tex file only')
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
      setError(e instanceof Error ? e.message : 'upload failed')
    } finally {
      setLoading(false)
    }
  }, [])

  const handleDrop = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragOver(false)

    const file = e.dataTransfer.files[0]
    if (file && isTexFile(file)) {
      void handleFileSelect(file)
    } else {
      setError('please upload a .tex file')
    }
  }, [handleFileSelect])

  const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      void handleFileSelect(file)
    }
  }, [handleFileSelect])

  const openFilePicker = useCallback(() => {
    fileInputRef.current?.click()
  }, [])

  const handleUploadKeydown = useCallback((e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      openFilePicker()
    }
  }, [openFilePicker])

  const handleJobSubmit = async () => {
    if (!jobDescription.trim()) {
      setError('please enter a job description')
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
      setError(e instanceof Error ? e.message : 'failed to save job description')
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

      const data: OptimizeApiResponse = await res.json()
      setOptimizedLatex(data.optimized_latex)
      setChangesSummary(data.changes_summary)
      setStep('result')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'optimization failed')
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
      setError(e instanceof Error ? e.message : 'cover letter generation failed')
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

      const data: ResumePdfApiResponse = await res.json()

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
      setError(e instanceof Error ? e.message : 'cover letter pdf generation failed')
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

      const data: ResumePdfApiResponse = await res.json()

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

      if (data.company_name && data.folder_path) {
        setSavedPackage({
          company_name: data.company_name,
          folder_path: data.folder_path,
          resume_pdf_path: data.resume_pdf_path,
          cover_letter_latex: coverLetter,
          optimized_latex: optimizedLatex,
          changes_summary: changesSummary,
          pdf_warnings: data.pdf_warnings,
          tex_files_deleted: data.tex_files_deleted ?? false,
        })
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'pdf generation failed')
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
      setError(e instanceof Error ? e.message : 'combined generation failed')
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
    <Flex minH="100vh" direction="column" align="center">
      <Box as="main" maxW="4xl" w="full" flex="1" px={{ base: 6, md: 14 }} py={{ base: 8, md: 10 }}>
        <Stack spacing={8} w="full" align="center" textAlign="center">
          {/* Title */}
          <Heading size="lg" mt={2}>targeted resume tailoring</Heading>

          {/* Progress */}
          <Flex gap={1} wrap="wrap" justify="center" align="center" aria-label="Progress">
            {STEPS.map((phase, index) => {
              const isComplete = currentStepIndex > index
              const isActive = currentStepIndex === index

              return (
                <HStack key={phase} spacing={0}>
                  <Text
                    fontSize="md"
                    fontWeight={isActive ? 'bold' : 'normal'}
                    color={isActive ? 'ink.900' : isComplete ? 'ink.700' : 'ink.400'}
                    textDecoration={isActive ? 'underline' : 'none'}
                    sx={isActive ? { textDecorationStyle: 'wavy', textUnderlineOffset: '4px' } : undefined}
                  >
                    {isComplete ? '✓ ' : `${index + 1}. `}{STEP_LABELS[phase]}
                  </Text>
                  {index < STEPS.length - 1 && (
                    <Text color="ink.300" mx={2} fontStyle="normal">—</Text>
                  )}
                </HStack>
              )
            })}
          </Flex>

          {/* Error */}
          {error && (
            <Alert
              status="error"
              bg="transparent"
              color="ink.900"
              borderLeft="3px solid"
              borderColor="red.400"
              borderRadius={0}
              w="full"
              px={4}
              py={3}
            >
              <AlertIcon />
              {error}
            </Alert>
          )}

          {/* ── Upload Step ──────────────────────── */}
          {step === 'upload' && (
            <Stack spacing={5} w="full" align="center">
              <Box textAlign="center">
                <Heading size="md" mb={2}>upload resume</Heading>
                <Text color="ink.700">upload a .tex file to start tailoring your resume.</Text>
              </Box>

              <Box
                w="full"
                minH="200px"
                borderWidth="2px"
                borderStyle="dashed"
                borderColor={dragOver ? 'ink.700' : resumeFile ? 'ink.500' : 'ink.300'}
                display="grid"
                placeItems="center"
                textAlign="center"
                px={4}
                cursor="pointer"
                transition="border-color 180ms ease"
                bg="transparent"
                _hover={{ borderColor: 'ink.500' }}
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
                aria-label="upload latex resume"
              >
                <input
                  ref={fileInputRef}
                  id="file-input"
                  type="file"
                  accept=".tex"
                  onChange={handleFileInput}
                  style={{ display: 'none' }}
                />

                {loading ? (
                  <Stack spacing={3} align="center">
                    <Spinner size="lg" color="ink.900" thickness="3px" />
                    <Text fontWeight="semibold">uploading...</Text>
                  </Stack>
                ) : resumeFile ? (
                  <Stack spacing={2} align="center">
                    <FileIcon boxSize={7} />
                    <Text fontWeight="bold">{resumeFile.name}</Text>
                    <Text color="ink.700">{(resumeFile.size / 1024).toFixed(1)} KB</Text>
                  </Stack>
                ) : (
                  <Stack spacing={2} align="center">
                    <UploadIcon boxSize={8} />
                    <Text fontWeight="bold">drop your file here</Text>
                    <Text color="ink.700">or click to browse</Text>
                  </Stack>
                )}
              </Box>

              {resumeContent && (
                <Stack spacing={2} w="full">
                  <Heading size="sm">preview</Heading>
                  <Box {...codeBlock} maxH="280px">
                    {resumeContent.slice(0, 2000)}
                    {resumeContent.length > 2000 ? '...' : ''}
                  </Box>
                </Stack>
              )}
            </Stack>
          )}

          {/* ── Job Description Step ─────────────── */}
          {step === 'job' && (
            <Stack spacing={5} w="full" align="center">
              <Box textAlign="center">
                <Heading size="md" mb={2}>job description</Heading>
                <Text color="ink.700">paste the full description to align your resume and cover letter.</Text>
              </Box>

              {hasSavedResume && (
                <Text color="ink.600" fontSize="sm">
                  using your saved base resume. replace it if needed.
                </Text>
              )}

              <Textarea
                w="full"
                minH="300px"
                placeholder="paste the full job description here..."
                value={jobDescription}
                onChange={(e) => setJobDescription(e.target.value)}
              />

              <Flex gap={3} wrap="wrap" justify="center">
                <Button variant="subtle" onClick={() => setStep('upload')}>
                  {hasSavedResume ? 'replace resume' : 'back'}
                </Button>
                <Button onClick={handleJobSubmit} isDisabled={loading} leftIcon={loading ? <Spinner size="sm" /> : undefined}>
                  {loading ? 'saving' : 'continue'}
                </Button>
              </Flex>
            </Stack>
          )}

          {/* ── Optimize Step ────────────────────── */}
          {step === 'optimize' && (
            <Stack spacing={6} w="full" align="center" textAlign="center">
              <SparkleIcon boxSize={8} color="ink.700" />

              <Box>
                <Heading size="md" mb={2}>ready to optimize</Heading>
                <Text color="ink.700" maxW="42ch" mx="auto">
                  run the optimization to produce an ats-focused version for this role.
                </Text>
              </Box>

              <List spacing={3} maxW="560px" textAlign="left" color="ink.700" fontSize="md">
                <ListItem>
                  <ListIcon as={CheckIcon} color="ink.900" />
                  match responsibilities to your strongest impact points
                </ListItem>
                <ListItem>
                  <ListIcon as={CheckIcon} color="ink.900" />
                  introduce relevant keywords naturally
                </ListItem>
                <ListItem>
                  <ListIcon as={CheckIcon} color="ink.900" />
                  keep formatting ats-safe
                </ListItem>
                <ListItem>
                  <ListIcon as={CheckIcon} color="ink.900" />
                  tighten phrasing for clarity and impact
                </ListItem>
              </List>

              <Flex gap={3} wrap="wrap" justify="center">
                <Button variant="subtle" onClick={() => setStep('job')}>back</Button>
                <Button onClick={handleOptimize} isDisabled={loading} leftIcon={loading ? <Spinner size="sm" /> : undefined}>
                  {loading ? 'optimizing' : 'optimize resume'}
                </Button>
              </Flex>
            </Stack>
          )}

          {/* ── Result Step ──────────────────────── */}
          {step === 'result' && (
            <Stack spacing={6} w="full" align="center" textAlign="center">
              <Stack spacing={3} align="center">
                <Box>
                  <Heading size="md" mb={2}>optimized result</Heading>
                  <Text color="ink.700">review updates and export your final files.</Text>
                </Box>
                <Button variant="subtle" onClick={handleReset}>start over</Button>
              </Stack>

              {changesSummary && (
                <Stack spacing={2} w="full">
                  <Heading size="sm">changes</Heading>
                  <Box
                    p={4}
                    borderLeft="2px solid"
                    borderColor="ink.300"
                    whiteSpace="pre-wrap"
                    color="ink.700"
                    textAlign="left"
                  >
                    {changesSummary}
                  </Box>
                </Stack>
              )}

              <Stack spacing={2} w="full">
                <Heading size="sm">optimized latex</Heading>
                <Box {...codeBlock}>
                  {optimizedLatex}
                </Box>
              </Stack>

              <Flex gap={3} wrap="wrap" justify="center">
                <Button
                  onClick={handleDownloadPdf}
                  isDisabled={loading}
                  leftIcon={loading ? <Spinner size="sm" /> : <DownloadIcon boxSize={4} />}
                >
                  {loading ? 'building pdf' : 'download pdf'}
                </Button>

                <Button
                  onClick={handleGenerateAndSavePackage}
                  isDisabled={loading}
                  leftIcon={loading ? <Spinner size="sm" /> : <SparkleIcon boxSize={4} />}
                >
                  {savingPackage ? 'generating & saving' : 'generate + save resume & cover letter'}
                </Button>

                <Button
                  variant="subtle"
                  onClick={handleGenerateCoverLetter}
                  isDisabled={loading || Boolean(coverLetter)}
                  leftIcon={coverLetter ? <CheckIcon boxSize={4} /> : <SparkleIcon boxSize={4} />}
                >
                  {coverLetter ? 'cover letter generated' : 'generate cover letter'}
                </Button>

                {coverLetter && (
                  <Button
                    onClick={handleDownloadCoverLetterPdf}
                    isDisabled={loading}
                    leftIcon={loading ? <Spinner size="sm" /> : <DownloadIcon boxSize={4} />}
                  >
                    {loading ? 'building pdf' : 'download cover letter pdf'}
                  </Button>
                )}
              </Flex>

              {savedPackage && (
                <Stack spacing={2} w="full">
                  <Heading size="sm">saved application package</Heading>
                  <Box
                    {...codeBlock}
                    fontSize="sm"
                    fontFamily="inherit"
                  >
                    {[
                      `company: ${savedPackage.company_name}`,
                      `folder: ${savedPackage.folder_path}`,
                      savedPackage.tex_files_deleted ? 'temporary .tex files: deleted' : 'temporary .tex files: not fully deleted',
                      savedPackage.resume_pdf_path ? `resume (.pdf): ${savedPackage.resume_pdf_path}` : '',
                      savedPackage.cover_letter_pdf_path ? `cover letter (.pdf): ${savedPackage.cover_letter_pdf_path}` : '',
                    ].filter(Boolean).join('\n')}
                  </Box>
                  {savedPackage.pdf_warnings && savedPackage.pdf_warnings.length > 0 && (
                    <Text whiteSpace="pre-wrap" color="ink.700" textAlign="left">{savedPackage.pdf_warnings.join('\n')}</Text>
                  )}
                </Stack>
              )}

              {coverLetter && (
                <Stack spacing={2} w="full" align="center">
                  <Heading size="sm">cover letter latex</Heading>
                  <Box {...codeBlock} w="full">
                    {coverLetter}
                  </Box>

                  <Button
                    variant="subtle"
                    w="fit-content"
                    onClick={() => {
                      void navigator.clipboard.writeText(coverLetter)
                    }}
                  >
                    copy latex to clipboard
                  </Button>
                </Stack>
              )}
            </Stack>
          )}
        </Stack>

        {/* Footer */}
        <Box mt={12} pt={6} borderTop="1px solid" borderColor="ink.200" textAlign="center">
          <Text color="ink.500" fontSize="sm">
            built for focused, minimal resume workflow.
          </Text>
        </Box>
      </Box>
    </Flex>
  )
}

export default App
