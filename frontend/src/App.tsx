import { useState, useCallback, useEffect, useRef } from 'react'
import { onAuthStateChanged, signInWithPopup, signOut, type User } from 'firebase/auth'
import { auth, googleProvider } from './firebase'
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
  pdf_warnings?: string[]
}

type Step = 'upload' | 'job' | 'optimize' | 'result'
type DownloadOption = 'resume' | 'resume_cv'

const STEPS: Step[] = ['upload', 'job', 'optimize', 'result']

const STEP_LABELS: Record<Step, string> = {
  upload: 'upload',
  job: 'job',
  optimize: 'optimize',
  result: 'result',
}

const isTexFile = (file: File): boolean => file.name.toLowerCase().endsWith('.tex')

const readApiError = async (res: Response): Promise<string> => {
  try {
    const err = await res.json() as Partial<ApiError>
    if (typeof err.error === 'string' && err.error.trim().length > 0) {
      return err.error
    }
  } catch {
    // Fall through to generic response message.
  }
  return `request failed (${res.status})`
}

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
  const [firebaseUser, setFirebaseUser] = useState<User | null>(null)
  const [authLoading, setAuthLoading] = useState(true)
  const [waitlistStatus, setWaitlistStatus] = useState<'pending' | 'invited' | 'approved' | null>(null)
  const [waitlistChecked, setWaitlistChecked] = useState(false)

  useEffect(() => {
    return onAuthStateChanged(auth, (u) => {
      setFirebaseUser(u)
      setAuthLoading(false)
      if (!u) {
        setWaitlistStatus(null)
        setWaitlistChecked(false)
      }
    })
  }, [])

  const [step, setStep] = useState<Step>('upload')
  const [resumeFile, setResumeFile] = useState<File | null>(null)
  const [resumeContent, setResumeContent] = useState<string>('')
  const [jobDescription, setJobDescription] = useState<string>('')
  const [loading, setLoading] = useState<boolean>(false)
  const [error, setError] = useState<string>('')
  const [dragOver, setDragOver] = useState<boolean>(false)
  const [hasSavedResume, setHasSavedResume] = useState<boolean>(false)
  const [selectedDownloadOption, setSelectedDownloadOption] = useState<DownloadOption | null>(null)

  const fileInputRef = useRef<HTMLInputElement>(null)
  const currentStepIndex = STEPS.indexOf(step)

  const apiFetch = useCallback(async (path: string, init?: RequestInit): Promise<Response> => {
    const token = firebaseUser ? await firebaseUser.getIdToken() : null
    const headers = new Headers(init?.headers ?? undefined)
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }
    return fetch(path, { ...init, headers })
  }, [firebaseUser])

  const handleLogout = useCallback(() => {
    void signOut(auth)
  }, [])

  useEffect(() => {
    if (!firebaseUser) {
      return
    }

    const checkWaitlist = async () => {
      try {
        const res = await apiFetch('/api/waitlist-status')
        if (!res.ok) return

        const data = await res.json() as { status: 'pending' | 'invited' | 'approved' }
        setWaitlistStatus(data.status)
      } catch {
        // Remain unchecked.
      } finally {
        setWaitlistChecked(true)
      }
    }

    void checkWaitlist()
  }, [apiFetch, firebaseUser])

  useEffect(() => {
    if (!firebaseUser || waitlistStatus !== 'approved') {
      return
    }

    const checkResumeStatus = async () => {
      try {
        const res = await apiFetch('/api/resume-status')
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
  }, [apiFetch, firebaseUser, waitlistStatus])

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

      const res = await apiFetch('/api/upload-resume', {
        method: 'POST',
        body: formData,
      })

      if (!res.ok) {
        throw new Error(await readApiError(res))
      }

      setHasSavedResume(true)
      setStep('job')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'upload failed')
    } finally {
      setLoading(false)
    }
  }, [apiFetch])

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

    try {
      const res = await apiFetch('/api/job-description', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ job_description: jobDescription }),
      })

      if (!res.ok) {
        throw new Error(await readApiError(res))
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

    try {
      const res = await apiFetch('/api/optimize', { method: 'POST' })

      if (!res.ok) {
        throw new Error(await readApiError(res))
      }

      await res.json() as Promise<OptimizeApiResponse>
      setSelectedDownloadOption(null)
      setStep('result')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'optimization failed')
    } finally {
      setLoading(false)
    }
  }

  const downloadPdfFromBase64 = useCallback((pdfBase64: string, filename: string) => {
    const byteCharacters = atob(pdfBase64)
    const byteNumbers = new Array(byteCharacters.length)
    for (let i = 0; i < byteCharacters.length; i += 1) {
      byteNumbers[i] = byteCharacters.charCodeAt(i)
    }

    const byteArray = new Uint8Array(byteNumbers)
    const blob = new Blob([byteArray], { type: 'application/pdf' })
    const url = URL.createObjectURL(blob)

    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }, [])

  const handleDownloadResumePdf = async () => {
    setSelectedDownloadOption('resume')
    setLoading(true)
    setError('')

    try {
      const res = await apiFetch('/api/generate-pdf', { method: 'POST' })

      if (!res.ok) {
        throw new Error(await readApiError(res))
      }

      const data: ResumePdfApiResponse = await res.json()
      downloadPdfFromBase64(data.pdf_base64, data.filename)
    } catch (e) {
      setSelectedDownloadOption(null)
      setError(e instanceof Error ? e.message : 'pdf generation failed')
    } finally {
      setLoading(false)
    }
  }

  const handleDownloadResumeAndCv = async () => {
    setSelectedDownloadOption('resume_cv')
    setLoading(true)
    setError('')

    try {
      const packageRes = await apiFetch('/api/generate-application-package', { method: 'POST' })

      if (!packageRes.ok) {
        throw new Error(await readApiError(packageRes))
      }

      const packageData: ApplicationPackageResponse = await packageRes.json()

      const [resumeRes, cvRes] = await Promise.all([
        apiFetch('/api/generate-pdf', { method: 'POST' }),
        apiFetch('/api/generate-cover-letter-pdf', { method: 'POST' }),
      ])

      if (!resumeRes.ok) {
        throw new Error(await readApiError(resumeRes))
      }
      if (!cvRes.ok) {
        throw new Error(await readApiError(cvRes))
      }

      const [resumeData, cvData] = await Promise.all([
        resumeRes.json() as Promise<ResumePdfApiResponse>,
        cvRes.json() as Promise<ResumePdfApiResponse>,
      ])

      downloadPdfFromBase64(resumeData.pdf_base64, resumeData.filename)
      downloadPdfFromBase64(cvData.pdf_base64, cvData.filename)

      if (packageData.pdf_warnings && packageData.pdf_warnings.length > 0) {
        setError(packageData.pdf_warnings.join('\n'))
      }
    } catch (e) {
      setSelectedDownloadOption(null)
      setError(e instanceof Error ? e.message : 'combined generation failed')
    } finally {
      setLoading(false)
    }
  }

  if (authLoading) {
    return (
      <Flex minH="100vh" align="center" justify="center" direction="column" gap={4}>
        <Spinner size="lg" color="ink.900" thickness="3px" />
        <Text color="ink.700">loading session...</Text>
      </Flex>
    )
  }

  if (!firebaseUser) {
    return (
      <Flex minH="100vh" align="center" justify="center" px={6}>
        <Stack spacing={6} w="full" maxW="lg" align="center" textAlign="center">
          <Heading size="lg">resume tailoring</Heading>
          <Text color="ink.700">
            sign in to access your resume workspace.
          </Text>
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
          <Button
            onClick={() => {
              setError('')
              signInWithPopup(auth, googleProvider).catch((err) => {
                setError(err instanceof Error ? err.message : 'sign in failed')
              })
            }}
          >
            sign in with Google
          </Button>
        </Stack>
      </Flex>
    )
  }

  if (!waitlistChecked) {
    return (
      <Flex minH="100vh" align="center" justify="center" direction="column" gap={4}>
        <Spinner size="lg" color="ink.900" thickness="3px" />
        <Text color="ink.700">checking access...</Text>
      </Flex>
    )
  }

  if (waitlistStatus === 'pending') {
    return (
      <Flex minH="100vh" align="center" justify="center" px={6}>
        <Stack spacing={6} w="full" maxW="lg" align="center" textAlign="center">
          <Heading size="lg">you're on the waitlist</Heading>
          <Text color="ink.700">
            thanks for signing up. we'll let you know when your account is ready.
          </Text>
          <Text color="ink.500" fontSize="sm">
            signed in as {firebaseUser.email ?? firebaseUser.displayName ?? 'unknown'}
          </Text>
          <Button size="sm" variant="subtle" onClick={handleLogout}>
            log out
          </Button>
        </Stack>
      </Flex>
    )
  }

  if (waitlistStatus === 'invited') {
    return (
      <Flex minH="100vh" align="center" justify="center" px={6}>
        <Stack spacing={6} w="full" maxW="lg" align="center" textAlign="center">
          <Heading size="lg">you've been invited</Heading>
          <Text color="ink.700">
            your invitation is being processed. you'll have full access shortly.
          </Text>
          <Text color="ink.500" fontSize="sm">
            signed in as {firebaseUser.email ?? firebaseUser.displayName ?? 'unknown'}
          </Text>
          <Button size="sm" variant="subtle" onClick={handleLogout}>
            log out
          </Button>
        </Stack>
      </Flex>
    )
  }

  if (waitlistStatus !== 'approved') {
    return (
      <Flex minH="100vh" align="center" justify="center" direction="column" gap={4}>
        <Text color="ink.700">unable to verify access. please try again later.</Text>
        <Button size="sm" variant="subtle" onClick={handleLogout}>
          log out
        </Button>
      </Flex>
    )
  }

  return (
    <Flex minH="100vh" direction="column" align="center">
      <Box as="main" maxW="4xl" w="full" flex="1" px={{ base: 6, md: 14 }} py={{ base: 8, md: 10 }}>
        <Stack spacing={8} w="full" align="center" textAlign="center">
          <HStack w="full" justify="space-between" align="center" fontSize="sm">
            <Text color="ink.600" noOfLines={1}>
              {firebaseUser.email ?? firebaseUser.displayName ?? 'signed in'}
            </Text>
            <Button size="sm" variant="subtle" onClick={handleLogout}>
              log out
            </Button>
          </HStack>

          {/* Title */}
          <Heading size="lg">targeted resume tailoring</Heading>

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
                    {isComplete && '✓ '}{STEP_LABELS[phase]}
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
              <Box>
                <Heading size="md" mb={2}>optimized result</Heading>
                <Text color="ink.700">choose one download option.</Text>
              </Box>

              <Stack spacing={3} w="full" maxW="460px">
                {(['resume', 'resume_cv'] as DownloadOption[]).map((option) => {
                  const isHidden = selectedDownloadOption !== null && selectedDownloadOption !== option
                  const isSelected = selectedDownloadOption === option

                  return (
                    <Box
                      key={option}
                      w="full"
                      maxH={isHidden ? '0px' : '80px'}
                      opacity={isHidden ? 0 : 1}
                      transform={isHidden ? 'translateY(-4px) scale(0.98)' : 'translateY(0) scale(1)'}
                      overflow="hidden"
                      pointerEvents={isHidden ? 'none' : 'auto'}
                      transition="max-height 220ms ease, opacity 220ms ease, transform 220ms ease"
                    >
                      <Button
                        onClick={option === 'resume' ? handleDownloadResumePdf : handleDownloadResumeAndCv}
                        isDisabled={loading || (selectedDownloadOption !== null && !isSelected)}
                        leftIcon={loading && isSelected ? <Spinner size="sm" /> : <DownloadIcon boxSize={4} />}
                        w="full"
                        size="lg"
                      >
                        {option === 'resume'
                          ? (loading && isSelected ? 'downloading resume' : 'download resume (pdf)')
                          : (loading && isSelected ? 'downloading resume + cv' : 'download resume + cv')}
                      </Button>
                    </Box>
                  )
                })}
              </Stack>
            </Stack>
          )}
        </Stack>

      </Box>
    </Flex>
  )
}

export default App
