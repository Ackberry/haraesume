import { useState, useCallback, useEffect, useRef } from 'react'
import {
  Alert,
  AlertIcon,
  Box,
  Button,
  Container,
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

const cardShell = {
  borderWidth: '1px',
  borderColor: 'ink.200',
  borderRadius: '20px',
  bgGradient: 'linear(155deg, whiteAlpha.950 0%, pastel.100 100%)',
  boxShadow: '0 10px 26px rgba(0, 0, 0, 0.06)',
  p: { base: 5, md: 6 },
  w: 'full',
  textAlign: 'center',
}

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
      void handleFileSelect(file)
    } else {
      setError('Please upload a .tex file')
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
    <Flex minH="100vh" direction="column">
      <Box
        borderBottomWidth="1px"
        borderColor="ink.200"
        bgGradient="linear(180deg, whiteAlpha.900 0%, pastel.100 100%)"
        backdropFilter="blur(8px)"
      >
        <Container maxW="4xl" py={4}>
          <Stack spacing={4} align="center" textAlign="center">
            <Flex justify="center" align="center" gap={3} wrap="wrap" w="full">
              <Box>
                <Text
                  fontSize="xs"
                  textTransform="uppercase"
                  letterSpacing="0.08em"
                  fontWeight="bold"
                  color="ink.600"
                >
                  Resume Optimizer
                </Text>
                <Heading size="md" fontWeight="bold">Targeted Resume Tailoring</Heading>
              </Box>
            </Flex>

            <Flex gap={2} wrap="wrap" justify="center" aria-label="Progress">
              {STEPS.map((phase, index) => {
                const isComplete = currentStepIndex > index
                const isActive = currentStepIndex === index

                return (
                  <HStack
                    key={phase}
                    spacing={2}
                    px={3}
                    py={2}
                    borderRadius="full"
                    borderWidth="1px"
                    borderColor={isActive ? 'ink.800' : isComplete ? 'ink.400' : 'ink.200'}
                    bgGradient={isActive
                      ? 'linear(135deg, pastel.300, pastel.100)'
                      : isComplete
                        ? 'linear(135deg, pastel.400, pastel.100)'
                        : 'linear(135deg, white, pastel.50)'}
                    color="ink.900"
                  >
                    <Flex
                      h={5}
                      w={5}
                      borderRadius="full"
                      borderWidth="1px"
                      borderColor="currentColor"
                      align="center"
                      justify="center"
                      fontSize="xs"
                    >
                      {isComplete ? <CheckIcon boxSize={3} /> : index + 1}
                    </Flex>
                    <Text fontSize="sm" fontWeight="semibold" display={{ base: 'none', sm: 'block' }}>
                      {STEP_LABELS[phase]}
                    </Text>
                  </HStack>
                )
              })}
            </Flex>
          </Stack>
        </Container>
      </Box>

      <Container maxW="4xl" py={{ base: 4, md: 6 }} flex="1">
        <Stack spacing={4} align="center">
          {error && (
            <Alert
              status="error"
              borderRadius="14px"
              borderWidth="1px"
              borderColor="red.200"
              bg="red.50"
              w="full"
              justifyContent="center"
              textAlign="center"
            >
              <AlertIcon />
              {error}
            </Alert>
          )}

          {step === 'upload' && (
            <Box {...cardShell}>
              <Stack spacing={4} align="center">
                <Box>
                  <Heading size="md" mb={1}>Upload Resume</Heading>
                  <Text color="ink.700">Upload a `.tex` file to start tailoring your resume.</Text>
                </Box>

                <Box
                  w="full"
                  minH="230px"
                  borderWidth="1.5px"
                  borderStyle="dashed"
                  borderColor={dragOver ? 'ink.700' : resumeFile ? 'ink.500' : 'ink.300'}
                  borderRadius="18px"
                  display="grid"
                  placeItems="center"
                  textAlign="center"
                  px={4}
                  cursor="pointer"
                  transition="all 180ms ease"
                  bgGradient={dragOver
                    ? 'linear(130deg, pastel.200, pastel.400)'
                    : resumeFile
                      ? 'linear(130deg, pastel.400, pastel.100)'
                      : 'linear(130deg, white, pastel.100)'}
                  _hover={{ borderColor: 'ink.600' }}
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
                      <Text fontWeight="semibold">Uploading...</Text>
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
                      <Text fontWeight="bold">Drop your file here</Text>
                      <Text color="ink.700">or click to browse</Text>
                    </Stack>
                  )}
                </Box>

                {resumeContent && (
                  <Stack spacing={2} w="full">
                    <Heading size="sm">Preview</Heading>
                    <Box
                      as="pre"
                      p={4}
                      borderRadius="14px"
                      borderWidth="1px"
                      borderColor="ink.200"
                      bgGradient="linear(155deg, white, pastel.50)"
                      fontSize="xs"
                      fontFamily="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
                      lineHeight="1.55"
                      maxH="280px"
                      overflow="auto"
                      whiteSpace="pre-wrap"
                      wordBreak="break-word"
                      textAlign="left"
                    >
                      {resumeContent.slice(0, 2000)}
                      {resumeContent.length > 2000 ? '...' : ''}
                    </Box>
                  </Stack>
                )}
              </Stack>
            </Box>
          )}

          {step === 'job' && (
            <Box {...cardShell}>
              <Stack spacing={4} align="center">
                <Box>
                  <Heading size="md" mb={1}>Job Description</Heading>
                  <Text color="ink.700">Paste the full description to align your resume and cover letter.</Text>
                </Box>

                {hasSavedResume && (
                  <Box
                    borderWidth="1px"
                    borderColor="ink.200"
                    borderRadius="12px"
                    bgGradient="linear(145deg, pastel.100, white)"
                    px={3}
                    py={2}
                    color="ink.700"
                    fontSize="sm"
                  >
                    Using your saved base resume. Replace it if needed.
                  </Box>
                )}

                <Textarea
                  w="full"
                  minH="300px"
                  placeholder="Paste the full job description here..."
                  value={jobDescription}
                  onChange={(e) => setJobDescription(e.target.value)}
                />

                <Flex gap={3} wrap="wrap" justify="center">
                  <Button variant="subtle" onClick={() => setStep('upload')}>
                    {hasSavedResume ? 'Replace Resume' : 'Back'}
                  </Button>
                  <Button onClick={handleJobSubmit} isDisabled={loading} leftIcon={loading ? <Spinner size="sm" /> : undefined}>
                    {loading ? 'Saving' : 'Continue'}
                  </Button>
                </Flex>
              </Stack>
            </Box>
          )}

          {step === 'optimize' && (
            <Box {...cardShell}>
              <Stack spacing={5} align="center" textAlign="center">
                <Flex
                  h={12}
                  w={12}
                  borderRadius="full"
                  align="center"
                  justify="center"
                  borderWidth="1px"
                  borderColor="ink.200"
                  bgGradient="linear(150deg, pastel.100, pastel.400)"
                >
                  <SparkleIcon boxSize={6} />
                </Flex>

                <Box>
                  <Heading size="md" mb={1}>Ready to Optimize</Heading>
                  <Text color="ink.700" maxW="42ch">
                    Run the optimization to produce an ATS-focused version for this role.
                  </Text>
                </Box>

                <List spacing={2} maxW="560px" textAlign="left" color="ink.700">
                  <ListItem>
                    <ListIcon as={CheckIcon} color="ink.900" />
                    Match responsibilities to your strongest impact points
                  </ListItem>
                  <ListItem>
                    <ListIcon as={CheckIcon} color="ink.900" />
                    Introduce relevant keywords naturally
                  </ListItem>
                  <ListItem>
                    <ListIcon as={CheckIcon} color="ink.900" />
                    Keep formatting ATS-safe
                  </ListItem>
                  <ListItem>
                    <ListIcon as={CheckIcon} color="ink.900" />
                    Tighten phrasing for clarity and impact
                  </ListItem>
                </List>

                <Flex gap={3} wrap="wrap" justify="center">
                  <Button variant="subtle" onClick={() => setStep('job')}>Back</Button>
                  <Button onClick={handleOptimize} isDisabled={loading} leftIcon={loading ? <Spinner size="sm" /> : undefined}>
                    {loading ? 'Optimizing' : 'Optimize Resume'}
                  </Button>
                </Flex>
              </Stack>
            </Box>
          )}

          {step === 'result' && (
            <Box {...cardShell}>
              <Stack spacing={5} align="center" textAlign="center">
                <Stack spacing={3} align="center">
                  <Box>
                    <Heading size="md" mb={1}>Optimized Result</Heading>
                    <Text color="ink.700">Review updates and export your final files.</Text>
                  </Box>
                  <Button variant="subtle" onClick={handleReset}>Start Over</Button>
                </Stack>

                {changesSummary && (
                  <Stack spacing={2} w="full">
                    <Heading size="sm">Changes</Heading>
                    <Box
                      p={4}
                      borderRadius="14px"
                      borderWidth="1px"
                      borderColor="ink.200"
                      bgGradient="linear(145deg, pastel.100, white)"
                      whiteSpace="pre-wrap"
                      color="ink.700"
                    >
                      {changesSummary}
                    </Box>
                  </Stack>
                )}

                <Stack spacing={2} w="full">
                  <Heading size="sm">Optimized LaTeX</Heading>
                  <Box
                    as="pre"
                    p={4}
                    borderRadius="14px"
                    borderWidth="1px"
                    borderColor="ink.200"
                    bgGradient="linear(155deg, white, pastel.50)"
                    fontSize="xs"
                    fontFamily="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
                    lineHeight="1.55"
                    maxH="360px"
                    overflow="auto"
                    whiteSpace="pre-wrap"
                    wordBreak="break-word"
                    textAlign="left"
                  >
                    {optimizedLatex}
                  </Box>
                </Stack>

                <Flex gap={3} wrap="wrap" justify="center">
                  <Button
                    onClick={handleDownloadPdf}
                    isDisabled={loading}
                    leftIcon={loading ? <Spinner size="sm" /> : <DownloadIcon boxSize={4} />}
                  >
                    {loading ? 'Building PDF' : 'Download PDF'}
                  </Button>

                  <Button
                    onClick={handleGenerateAndSavePackage}
                    isDisabled={loading}
                    leftIcon={loading ? <Spinner size="sm" /> : <SparkleIcon boxSize={4} />}
                  >
                    {savingPackage ? 'Generating & Saving' : 'Generate + Save Resume & Cover Letter'}
                  </Button>

                  <Button
                    variant="subtle"
                    onClick={handleGenerateCoverLetter}
                    isDisabled={loading || Boolean(coverLetter)}
                    leftIcon={coverLetter ? <CheckIcon boxSize={4} /> : <SparkleIcon boxSize={4} />}
                  >
                    {coverLetter ? 'Cover Letter Generated' : 'Generate Cover Letter'}
                  </Button>

                  {coverLetter && (
                    <Button
                      onClick={handleDownloadCoverLetterPdf}
                      isDisabled={loading}
                      leftIcon={loading ? <Spinner size="sm" /> : <DownloadIcon boxSize={4} />}
                    >
                      {loading ? 'Building PDF' : 'Download Cover Letter PDF'}
                    </Button>
                  )}
                </Flex>

                {savedPackage && (
                  <Stack spacing={2} w="full">
                    <Heading size="sm">Saved Application Package</Heading>
                    <Box
                      as="pre"
                      p={4}
                      borderRadius="14px"
                      borderWidth="1px"
                      borderColor="ink.200"
                      bgGradient="linear(145deg, pastel.100, white)"
                      fontSize="sm"
                      whiteSpace="pre-wrap"
                      textAlign="left"
                    >
                      {[
                        `Company: ${savedPackage.company_name}`,
                        `Folder: ${savedPackage.folder_path}`,
                        savedPackage.tex_files_deleted ? 'Temporary .tex files: deleted' : 'Temporary .tex files: not fully deleted',
                        savedPackage.resume_pdf_path ? `Resume (.pdf): ${savedPackage.resume_pdf_path}` : '',
                        savedPackage.cover_letter_pdf_path ? `Cover Letter (.pdf): ${savedPackage.cover_letter_pdf_path}` : '',
                      ].filter(Boolean).join('\n')}
                    </Box>
                    {savedPackage.pdf_warnings && savedPackage.pdf_warnings.length > 0 && (
                      <Text whiteSpace="pre-wrap" color="ink.700" textAlign="left">{savedPackage.pdf_warnings.join('\n')}</Text>
                    )}
                  </Stack>
                )}

                {coverLetter && (
                  <Stack spacing={2} w="full" align="center">
                    <Heading size="sm">Cover Letter LaTeX</Heading>
                    <Box
                      as="pre"
                      p={4}
                      borderRadius="14px"
                      borderWidth="1px"
                      borderColor="ink.200"
                      bgGradient="linear(155deg, white, pastel.50)"
                      fontSize="xs"
                      fontFamily="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
                      lineHeight="1.55"
                      maxH="360px"
                      overflow="auto"
                      whiteSpace="pre-wrap"
                      wordBreak="break-word"
                      textAlign="left"
                    >
                      {coverLetter}
                    </Box>

                    <Button
                      variant="subtle"
                      w="fit-content"
                      onClick={() => {
                        void navigator.clipboard.writeText(coverLetter)
                      }}
                    >
                      Copy LaTeX to Clipboard
                    </Button>
                  </Stack>
                )}
              </Stack>
            </Box>
          )}
        </Stack>
      </Container>

      <Box borderTopWidth="1px" borderColor="ink.200" bgGradient="linear(180deg, white, pastel.50)">
        <Container maxW="4xl" py={4}>
          <Text color="ink.600" fontSize="sm" textAlign="center">
            Built for focused, minimal resume workflow.
          </Text>
        </Container>
      </Box>
    </Flex>
  )
}

export default App
