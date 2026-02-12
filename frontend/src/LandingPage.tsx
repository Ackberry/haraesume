import { Box, Flex, Heading, Link, Stack, Text } from '@chakra-ui/react'

function LandingPage() {
  return (
    <Flex minH="100vh" direction="column" align="center" justify="center" position="relative">
      <Link
        href="https://github.com/ackberry"
        isExternal
        position="fixed"
        top={5}
        right={5}
        color="ink.500"
        _hover={{ color: 'ink.900' }}
        transition="color 180ms ease"
        aria-label="GitHub"
        zIndex={10}
      >
        github
      </Link>

      <Box as="main" maxW="4xl" w="full" px={{ base: 6, md: 14 }} py={{ base: 10, md: 16 }}>
        <Stack spacing={10} w="full" align="center" textAlign="center">
          {/* Hero */}
          <Stack spacing={4} align="center">
            <Heading size="2xl" letterSpacing="-0.03em">
              haraesume
            </Heading>
            <Text color="ink.600" maxW="48ch" mx="auto" lineHeight="1.8">
              create job-specific resume in seconds.
            </Text>
            <Text color="ink.600" maxW="48ch" mx="auto">
              access is currently invite-only.
            </Text>
          </Stack>
        </Stack>
      </Box>
    </Flex>
  )
}

export default LandingPage
