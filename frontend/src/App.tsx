import { useState, useCallback, useEffect, useRef } from 'react'
import { onAuthStateChanged, signInWithPopup, signOut, type User } from 'firebase/auth'
import { doc, getDoc, setDoc, serverTimestamp } from 'firebase/firestore'
import { auth, googleProvider, db } from './firebase'
import {
  Alert,
  AlertIcon,
  Box,
  Button,
  Flex,
  Heading,
  HStack,
  Icon,
  Input,
  List,
  ListIcon,
  ListItem,
  Spinner,
  Stack,
  Text,
  Textarea,
  CloseButton,
  type IconProps,
} from '@chakra-ui/react'

type AppMode = 'optimize' | 'build'

interface ApiError {
  error: string
}

interface BuilderHeading {
  name: string
  linkedin: string
  github: string
  email: string
  phone: string
  portfolio: string
}

interface BuilderEducation {
  school: string
  graduation: string
  degree: string
  location: string
  coursework: string
}

interface BuilderExperience {
  company: string
  dates: string
  title: string
  location: string
  bullets: string[]
}

interface BuilderProject {
  name: string
  link: string
  techStack: string
  bullets: string[]
}

interface BuilderSkills {
  languages: string
  frameworks: string
  databases: string
  infrastructure: string
}

interface BuilderLeadership {
  organization: string
  dates: string
  title: string
  location: string
}

const emptyHeading: BuilderHeading = { name: '', linkedin: '', github: '', email: '', phone: '', portfolio: '' }
const emptyEducation = (): BuilderEducation => ({ school: '', graduation: '', degree: '', location: '', coursework: '' })
const emptyExperience = (): BuilderExperience => ({ company: '', dates: '', title: '', location: '', bullets: [''] })
const emptyProject = (): BuilderProject => ({ name: '', link: '', techStack: '', bullets: [''] })
const emptySkills: BuilderSkills = { languages: '', frameworks: '', databases: '', infrastructure: '' }
const emptyLeadership = (): BuilderLeadership => ({ organization: '', dates: '', title: '', location: '' })

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

type Step = 'upload' | 'job' | 'optimize' | 'result'
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

const BackArrowIcon = (props: IconProps) => (
  <Icon viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" {...props}>
    <path d="M19 12H5M12 19l-7-7 7-7" />
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
  const [waitlistEmail, setWaitlistEmail] = useState('')
  const [waitlistSubmitted, setWaitlistSubmitted] = useState(false)
  const [waitlistSubmitting, setWaitlistSubmitting] = useState(false)

  useEffect(() => {
    return onAuthStateChanged(auth, (u) => {
      setFirebaseUser(u)
      setAuthLoading(false)
      if (u) {
        // Pre-fetch and cache the ID token so the first apiFetch doesn't block on it.
        void u.getIdToken()
      } else {
        setWaitlistStatus(null)
        setWaitlistChecked(false)
      }
    })
  }, [])

  const [appMode, setAppMode] = useState<AppMode>('optimize')

  // Builder state
  const [builderHeading, setBuilderHeading] = useState<BuilderHeading>({ ...emptyHeading })
  const [builderEducation, setBuilderEducation] = useState<BuilderEducation[]>([emptyEducation()])
  const [builderExperience, setBuilderExperience] = useState<BuilderExperience[]>([emptyExperience()])
  const [builderProjects, setBuilderProjects] = useState<BuilderProject[]>([emptyProject()])
  const [builderSkills, setBuilderSkills] = useState<BuilderSkills>({ ...emptySkills })
  const [builderLeadership, setBuilderLeadership] = useState<BuilderLeadership[]>([])
  const [builderLoading, setBuilderLoading] = useState(false)
  const [builderError, setBuilderError] = useState('')

  const [step, setStep] = useState<Step>('upload')
  const [resumeFile, setResumeFile] = useState<File | null>(null)
  const [resumeContent, setResumeContent] = useState<string>('')
  const [jobDescription, setJobDescription] = useState<string>('')
  const [loading, setLoading] = useState<boolean>(false)
  const [error, setError] = useState<string>('')
  const [dragOver, setDragOver] = useState<boolean>(false)
  const [hasSavedResume, setHasSavedResume] = useState<boolean>(false)
  const [showRetentionNotice, setShowRetentionNotice] = useState(false)
  const [changesSummary, setChangesSummary] = useState<string[]>([])

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

  const handleJoinWaitlist = useCallback(async () => {
    const email = waitlistEmail.trim().toLowerCase()
    if (!email || !email.includes('@')) {
      setError('please enter a valid email address')
      return
    }

    setWaitlistSubmitting(true)
    setError('')

    try {
      const docRef = doc(db, 'waitlist', email)
      const existing = await getDoc(docRef)
      if (!existing.exists()) {
        await setDoc(docRef, {
          email,
          status: 'pending',
          created_at: serverTimestamp(),
        })

        fetch('/api/waitlist/notify-signup', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email }),
        }).catch(() => {})
      }
      setWaitlistSubmitted(true)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'failed to join waitlist')
    } finally {
      setWaitlistSubmitting(false)
    }
  }, [waitlistEmail])

  useEffect(() => {
    if (!firebaseUser) {
      return
    }

    // Use cached waitlist status for instant UI while re-verifying in background.
    const cacheKey = `waitlist_status_${firebaseUser.uid}`
    const cached = localStorage.getItem(cacheKey)
    if (cached === 'approved' || cached === 'invited' || cached === 'pending') {
      setWaitlistStatus(cached)
      setWaitlistChecked(true)
    }

    const checkApproval = async () => {
      const statusRank = { pending: 0, invited: 1, approved: 2 } as const
      type WaitlistStatus = keyof typeof statusRank
      let best: WaitlistStatus = 'pending'
      let gotResponse = false

      // Run Firestore and backend checks in parallel — use whichever is better.
      const firestoreCheck = async () => {
        try {
          const email = (firebaseUser.email ?? '').toLowerCase()
          if (!email) return
          const docRef = doc(db, 'waitlist', email)
          const snap = await getDoc(docRef)
          if (snap.exists()) {
            const s = (snap.data().status as string ?? '').trim() as WaitlistStatus
            if (statusRank[s] !== undefined && statusRank[s] > statusRank[best]) {
              best = s
              gotResponse = true
            }
          }
        } catch { /* Firestore unavailable */ }
      }

      const backendCheck = async () => {
        try {
          const res = await apiFetch('/api/waitlist/status')
          if (res.ok) {
            const data = await res.json() as { status: string }
            const s = (data.status ?? '').trim() as WaitlistStatus
            if (statusRank[s] !== undefined && statusRank[s] > statusRank[best]) {
              best = s
              gotResponse = true
            }
          }
        } catch { /* Backend unavailable */ }
      }

      await Promise.all([firestoreCheck(), backendCheck()])

      if (gotResponse) {
        localStorage.setItem(cacheKey, best)
      } else if (cached) {
        // Both failed but we have a cache — keep it, don't overwrite.
        return
      }

      setWaitlistStatus(best)
      setWaitlistChecked(true)
    }

    void checkApproval()
  }, [firebaseUser, apiFetch])

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

  useEffect(() => {
    if (!firebaseUser || waitlistStatus !== 'approved') return
    const key = `retention_notice_dismissed_${firebaseUser.uid}`
    if (!localStorage.getItem(key)) {
      setShowRetentionNotice(true)
    }
  }, [firebaseUser, waitlistStatus])

  const dismissRetentionNotice = useCallback(() => {
    setShowRetentionNotice(false)
    if (firebaseUser) {
      localStorage.setItem(`retention_notice_dismissed_${firebaseUser.uid}`, '1')
    }
  }, [firebaseUser])

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

      const data: OptimizeApiResponse = await res.json()

      const bullets = (data.changes_summary ?? '')
        .split('\n')
        .map((line) => line.replace(/^[-•*]\s*/, '').trim())
        .filter(Boolean)
      setChangesSummary(bullets)

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
      setError(e instanceof Error ? e.message : 'pdf generation failed')
    } finally {
      setLoading(false)
    }
  }

  const handleBuilderGenerate = async () => {
    if (!builderHeading.name.trim() || !builderHeading.email.trim()) {
      setBuilderError('name and email are required')
      return
    }

    setBuilderLoading(true)
    setBuilderError('')

    try {
      const res = await apiFetch('/api/builder/generate-pdf', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          heading: builderHeading,
          education: builderEducation.filter((e) => e.school.trim()),
          experience: builderExperience.filter((e) => e.company.trim()).map((e) => ({
            ...e,
            bullets: e.bullets.filter((b) => b.trim()),
          })),
          projects: builderProjects.filter((p) => p.name.trim()).map((p) => ({
            ...p,
            bullets: p.bullets.filter((b) => b.trim()),
          })),
          skills: builderSkills,
          leadership: builderLeadership.filter((l) => l.organization.trim()),
        }),
      })

      if (!res.ok) {
        throw new Error(await readApiError(res))
      }

      const data: ResumePdfApiResponse = await res.json()
      downloadPdfFromBase64(data.pdf_base64, data.filename)
    } catch (e) {
      setBuilderError(e instanceof Error ? e.message : 'pdf generation failed')
    } finally {
      setBuilderLoading(false)
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

          {waitlistSubmitted ? (
            <Stack spacing={4} align="center">
              <Text color="ink.700">
                thanks! you've been added to the waitlist. we'll let you know when your account is ready.
              </Text>
              <Text color="ink.500" fontSize="sm">
                already approved? sign in below.
              </Text>
            </Stack>
          ) : (
            <Stack spacing={4} w="full" align="center">
              <Text color="ink.700">
                enter your email to join the waitlist.
              </Text>
              <HStack w="full" maxW="sm">
                <Input
                  placeholder="your@email.com"
                  value={waitlistEmail}
                  onChange={(e) => setWaitlistEmail(e.target.value)}
                  type="email"
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') void handleJoinWaitlist()
                  }}
                />
                <Button
                  onClick={() => void handleJoinWaitlist()}
                  isDisabled={waitlistSubmitting}
                  flexShrink={0}
                >
                  {waitlistSubmitting ? <Spinner size="sm" /> : 'join'}
                </Button>
              </HStack>
            </Stack>
          )}

          <Box w="full" maxW="sm" borderTop="1px solid" borderColor="ink.200" />

          <Stack spacing={2} align="center">
            <Text color="ink.500" fontSize="sm">
              already have access?
            </Text>
            <Button
              variant="subtle"
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

          {/* Mode Tabs */}
          <HStack spacing={2}>
            <Button
              size="sm"
              variant={appMode === 'optimize' ? 'solid' : 'subtle'}
              onClick={() => { setAppMode('optimize'); setBuilderError('') }}
            >
              optimize
            </Button>
            <Button
              size="sm"
              variant={appMode === 'build' ? 'solid' : 'subtle'}
              onClick={() => { setAppMode('build'); setError('') }}
            >
              build
            </Button>
          </HStack>

          {appMode === 'optimize' && (
            <>
              {/* Back Arrow */}
              {step !== 'upload' && (
                <Box w="full">
                  <Button
                    variant="subtle"
                    size="sm"
                    leftIcon={<BackArrowIcon boxSize={4} />}
                    onClick={() => {
                      const prevStep = STEPS[currentStepIndex - 1]
                      if (prevStep) setStep(prevStep)
                    }}
                    pl={0}
                  >
                    back
                  </Button>
                </Box>
              )}

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
                    <Text color="ink.700">paste the full description to align your resume.</Text>
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

                  {changesSummary.length > 0 && (
                    <Box w="full" maxW="560px" textAlign="left">
                      <Text fontWeight="semibold" mb={3} color="ink.800">changes made</Text>
                      <List spacing={2} color="ink.700" fontSize="sm">
                        {changesSummary.map((change, i) => (
                          <ListItem key={i}>
                            <ListIcon as={CheckIcon} color="ink.600" />
                            {change}
                          </ListItem>
                        ))}
                      </List>
                    </Box>
                  )}

                  <Button
                    onClick={handleDownloadResumePdf}
                    isDisabled={loading}
                    leftIcon={loading ? <Spinner size="sm" /> : <DownloadIcon boxSize={4} />}
                    w="full"
                    maxW="460px"
                    size="lg"
                  >
                    {loading ? 'downloading resume' : 'download resume (pdf)'}
                  </Button>
                </Stack>
              )}
            </>
          )}

          {/* ── Builder Mode ─────────────────────── */}
          {appMode === 'build' && (
            <Stack spacing={8} w="full" textAlign="left">
              <Heading size="lg" textAlign="center">build your resume</Heading>

              {builderError && (
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
                  {builderError}
                </Alert>
              )}

              {/* ── Personal Info ── */}
              <Stack spacing={3}>
                <Heading size="sm">personal info</Heading>
                <HStack spacing={3} flexWrap="wrap">
                  <Input flex="1" minW="200px" placeholder="full name *" value={builderHeading.name} onChange={(e) => setBuilderHeading({ ...builderHeading, name: e.target.value })} />
                  <Input flex="1" minW="200px" placeholder="email *" value={builderHeading.email} onChange={(e) => setBuilderHeading({ ...builderHeading, email: e.target.value })} />
                </HStack>
                <HStack spacing={3} flexWrap="wrap">
                  <Input flex="1" minW="200px" placeholder="phone" value={builderHeading.phone} onChange={(e) => setBuilderHeading({ ...builderHeading, phone: e.target.value })} />
                  <Input flex="1" minW="200px" placeholder="linkedin url" value={builderHeading.linkedin} onChange={(e) => setBuilderHeading({ ...builderHeading, linkedin: e.target.value })} />
                </HStack>
                <HStack spacing={3} flexWrap="wrap">
                  <Input flex="1" minW="200px" placeholder="github url" value={builderHeading.github} onChange={(e) => setBuilderHeading({ ...builderHeading, github: e.target.value })} />
                  <Input flex="1" minW="200px" placeholder="portfolio url" value={builderHeading.portfolio} onChange={(e) => setBuilderHeading({ ...builderHeading, portfolio: e.target.value })} />
                </HStack>
              </Stack>

              {/* ── Education ── */}
              <Stack spacing={3}>
                <HStack justify="space-between">
                  <Heading size="sm">education</Heading>
                  <Button size="xs" variant="subtle" onClick={() => setBuilderEducation([...builderEducation, emptyEducation()])}>+ add</Button>
                </HStack>
                {builderEducation.map((edu, i) => (
                  <Stack key={i} spacing={2} p={3} border="1px solid" borderColor="ink.200">
                    {builderEducation.length > 1 && (
                      <Flex justify="flex-end">
                        <Button size="xs" variant="subtle" color="red.500" onClick={() => setBuilderEducation(builderEducation.filter((_, j) => j !== i))}>remove</Button>
                      </Flex>
                    )}
                    <HStack spacing={3} flexWrap="wrap">
                      <Input flex="1" minW="200px" placeholder="school" value={edu.school} onChange={(e) => { const copy = [...builderEducation]; copy[i] = { ...edu, school: e.target.value }; setBuilderEducation(copy) }} />
                      <Input flex="1" minW="150px" placeholder="graduation (e.g. May 2027)" value={edu.graduation} onChange={(e) => { const copy = [...builderEducation]; copy[i] = { ...edu, graduation: e.target.value }; setBuilderEducation(copy) }} />
                    </HStack>
                    <HStack spacing={3} flexWrap="wrap">
                      <Input flex="1" minW="200px" placeholder="degree" value={edu.degree} onChange={(e) => { const copy = [...builderEducation]; copy[i] = { ...edu, degree: e.target.value }; setBuilderEducation(copy) }} />
                      <Input flex="1" minW="150px" placeholder="location" value={edu.location} onChange={(e) => { const copy = [...builderEducation]; copy[i] = { ...edu, location: e.target.value }; setBuilderEducation(copy) }} />
                    </HStack>
                    <Input placeholder="relevant coursework (comma-separated)" value={edu.coursework} onChange={(e) => { const copy = [...builderEducation]; copy[i] = { ...edu, coursework: e.target.value }; setBuilderEducation(copy) }} />
                  </Stack>
                ))}
              </Stack>

              {/* ── Experience ── */}
              <Stack spacing={3}>
                <HStack justify="space-between">
                  <Heading size="sm">experience</Heading>
                  <Button size="xs" variant="subtle" onClick={() => setBuilderExperience([...builderExperience, emptyExperience()])}>+ add</Button>
                </HStack>
                {builderExperience.map((exp, i) => (
                  <Stack key={i} spacing={2} p={3} border="1px solid" borderColor="ink.200">
                    {builderExperience.length > 1 && (
                      <Flex justify="flex-end">
                        <Button size="xs" variant="subtle" color="red.500" onClick={() => setBuilderExperience(builderExperience.filter((_, j) => j !== i))}>remove</Button>
                      </Flex>
                    )}
                    <HStack spacing={3} flexWrap="wrap">
                      <Input flex="1" minW="200px" placeholder="company" value={exp.company} onChange={(e) => { const copy = [...builderExperience]; copy[i] = { ...exp, company: e.target.value }; setBuilderExperience(copy) }} />
                      <Input flex="1" minW="150px" placeholder="dates (e.g. Jan 2025 -- Present)" value={exp.dates} onChange={(e) => { const copy = [...builderExperience]; copy[i] = { ...exp, dates: e.target.value }; setBuilderExperience(copy) }} />
                    </HStack>
                    <HStack spacing={3} flexWrap="wrap">
                      <Input flex="1" minW="200px" placeholder="title" value={exp.title} onChange={(e) => { const copy = [...builderExperience]; copy[i] = { ...exp, title: e.target.value }; setBuilderExperience(copy) }} />
                      <Input flex="1" minW="150px" placeholder="location" value={exp.location} onChange={(e) => { const copy = [...builderExperience]; copy[i] = { ...exp, location: e.target.value }; setBuilderExperience(copy) }} />
                    </HStack>
                    <Stack spacing={1}>
                      <Text fontSize="xs" color="ink.500">bullet points</Text>
                      {exp.bullets.map((bullet, bi) => (
                        <HStack key={bi} spacing={2}>
                          <Input flex="1" size="sm" placeholder="bullet point" value={bullet} onChange={(e) => {
                            const copy = [...builderExperience]
                            const bullets = [...exp.bullets]
                            bullets[bi] = e.target.value
                            copy[i] = { ...exp, bullets }
                            setBuilderExperience(copy)
                          }} />
                          {exp.bullets.length > 1 && (
                            <Button size="xs" variant="subtle" color="red.500" onClick={() => {
                              const copy = [...builderExperience]
                              copy[i] = { ...exp, bullets: exp.bullets.filter((_, j) => j !== bi) }
                              setBuilderExperience(copy)
                            }}>x</Button>
                          )}
                        </HStack>
                      ))}
                      <Button size="xs" variant="subtle" alignSelf="flex-start" onClick={() => {
                        const copy = [...builderExperience]
                        copy[i] = { ...exp, bullets: [...exp.bullets, ''] }
                        setBuilderExperience(copy)
                      }}>+ bullet</Button>
                    </Stack>
                  </Stack>
                ))}
              </Stack>

              {/* ── Projects ── */}
              <Stack spacing={3}>
                <HStack justify="space-between">
                  <Heading size="sm">projects</Heading>
                  <Button size="xs" variant="subtle" onClick={() => setBuilderProjects([...builderProjects, emptyProject()])}>+ add</Button>
                </HStack>
                {builderProjects.map((proj, i) => (
                  <Stack key={i} spacing={2} p={3} border="1px solid" borderColor="ink.200">
                    {builderProjects.length > 1 && (
                      <Flex justify="flex-end">
                        <Button size="xs" variant="subtle" color="red.500" onClick={() => setBuilderProjects(builderProjects.filter((_, j) => j !== i))}>remove</Button>
                      </Flex>
                    )}
                    <HStack spacing={3} flexWrap="wrap">
                      <Input flex="1" minW="200px" placeholder="project name" value={proj.name} onChange={(e) => { const copy = [...builderProjects]; copy[i] = { ...proj, name: e.target.value }; setBuilderProjects(copy) }} />
                      <Input flex="1" minW="200px" placeholder="link (optional)" value={proj.link} onChange={(e) => { const copy = [...builderProjects]; copy[i] = { ...proj, link: e.target.value }; setBuilderProjects(copy) }} />
                    </HStack>
                    <Input placeholder="tech stack (e.g. React, Go, PostgreSQL)" value={proj.techStack} onChange={(e) => { const copy = [...builderProjects]; copy[i] = { ...proj, techStack: e.target.value }; setBuilderProjects(copy) }} />
                    <Stack spacing={1}>
                      <Text fontSize="xs" color="ink.500">bullet points</Text>
                      {proj.bullets.map((bullet, bi) => (
                        <HStack key={bi} spacing={2}>
                          <Input flex="1" size="sm" placeholder="bullet point" value={bullet} onChange={(e) => {
                            const copy = [...builderProjects]
                            const bullets = [...proj.bullets]
                            bullets[bi] = e.target.value
                            copy[i] = { ...proj, bullets }
                            setBuilderProjects(copy)
                          }} />
                          {proj.bullets.length > 1 && (
                            <Button size="xs" variant="subtle" color="red.500" onClick={() => {
                              const copy = [...builderProjects]
                              copy[i] = { ...proj, bullets: proj.bullets.filter((_, j) => j !== bi) }
                              setBuilderProjects(copy)
                            }}>x</Button>
                          )}
                        </HStack>
                      ))}
                      <Button size="xs" variant="subtle" alignSelf="flex-start" onClick={() => {
                        const copy = [...builderProjects]
                        copy[i] = { ...proj, bullets: [...proj.bullets, ''] }
                        setBuilderProjects(copy)
                      }}>+ bullet</Button>
                    </Stack>
                  </Stack>
                ))}
              </Stack>

              {/* ── Technical Skills ── */}
              <Stack spacing={3}>
                <Heading size="sm">technical skills</Heading>
                <Input placeholder="languages (e.g. Python, Go, TypeScript)" value={builderSkills.languages} onChange={(e) => setBuilderSkills({ ...builderSkills, languages: e.target.value })} />
                <Input placeholder="frameworks & libraries" value={builderSkills.frameworks} onChange={(e) => setBuilderSkills({ ...builderSkills, frameworks: e.target.value })} />
                <Input placeholder="databases & servers" value={builderSkills.databases} onChange={(e) => setBuilderSkills({ ...builderSkills, databases: e.target.value })} />
                <Input placeholder="infrastructure & devops" value={builderSkills.infrastructure} onChange={(e) => setBuilderSkills({ ...builderSkills, infrastructure: e.target.value })} />
              </Stack>

              {/* ── Leadership ── */}
              <Stack spacing={3}>
                <HStack justify="space-between">
                  <Heading size="sm">leadership</Heading>
                  <Button size="xs" variant="subtle" onClick={() => setBuilderLeadership([...builderLeadership, emptyLeadership()])}>+ add</Button>
                </HStack>
                {builderLeadership.map((lead, i) => (
                  <Stack key={i} spacing={2} p={3} border="1px solid" borderColor="ink.200">
                    <Flex justify="flex-end">
                      <Button size="xs" variant="subtle" color="red.500" onClick={() => setBuilderLeadership(builderLeadership.filter((_, j) => j !== i))}>remove</Button>
                    </Flex>
                    <HStack spacing={3} flexWrap="wrap">
                      <Input flex="1" minW="200px" placeholder="organization" value={lead.organization} onChange={(e) => { const copy = [...builderLeadership]; copy[i] = { ...lead, organization: e.target.value }; setBuilderLeadership(copy) }} />
                      <Input flex="1" minW="150px" placeholder="dates" value={lead.dates} onChange={(e) => { const copy = [...builderLeadership]; copy[i] = { ...lead, dates: e.target.value }; setBuilderLeadership(copy) }} />
                    </HStack>
                    <HStack spacing={3} flexWrap="wrap">
                      <Input flex="1" minW="200px" placeholder="title" value={lead.title} onChange={(e) => { const copy = [...builderLeadership]; copy[i] = { ...lead, title: e.target.value }; setBuilderLeadership(copy) }} />
                      <Input flex="1" minW="150px" placeholder="location" value={lead.location} onChange={(e) => { const copy = [...builderLeadership]; copy[i] = { ...lead, location: e.target.value }; setBuilderLeadership(copy) }} />
                    </HStack>
                  </Stack>
                ))}
                {builderLeadership.length === 0 && (
                  <Text fontSize="sm" color="ink.500">no leadership entries yet. click + add to start.</Text>
                )}
              </Stack>

              {/* ── Generate Button ── */}
              <Flex justify="center" pt={4}>
                <Button
                  size="lg"
                  onClick={handleBuilderGenerate}
                  isDisabled={builderLoading}
                  leftIcon={builderLoading ? <Spinner size="sm" /> : <DownloadIcon boxSize={4} />}
                  w="full"
                  maxW="460px"
                >
                  {builderLoading ? 'generating pdf' : 'generate pdf'}
                </Button>
              </Flex>
            </Stack>
          )}
        </Stack>

      </Box>

      {showRetentionNotice && (
        <Box
          position="fixed"
          bottom={6}
          left="50%"
          transform="translateX(-50%)"
          w="auto"
          maxW="sm"
          bg="red.50"
          border="1px solid"
          borderColor="red.200"
          boxShadow="lg"
          px={5}
          py={4}
          zIndex="popover"
        >
          <Flex align="flex-start" gap={3}>
            <Text fontSize="sm" color="ink.700" flex="1">
              resumes are automatically deleted after 7 days.
            </Text>
            <CloseButton size="sm" onClick={dismissRetentionNotice} />
          </Flex>
        </Box>
      )}
    </Flex>
  )
}

export default App
