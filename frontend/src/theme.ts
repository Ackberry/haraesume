import { extendTheme, theme as baseTheme, type ThemeConfig } from '@chakra-ui/react'

const config: ThemeConfig = {
  initialColorMode: 'light',
  useSystemColorMode: false,
}

const mono = `'SF Mono', 'Fira Code', 'Fira Mono', 'Roboto Mono', 'Menlo', 'Consolas', 'DejaVu Sans Mono', monospace`

const fonts = {
  heading: mono,
  body: mono,
}

export const theme = extendTheme({
  config,
  fonts,
  fontSizes: {
    xs: '0.875rem',
    sm: '1rem',
    md: '1.125rem',
    lg: '1.3rem',
    xl: '1.55rem',
    '2xl': '1.95rem',
    '3xl': '2.35rem',
  },
  colors: {
    ...baseTheme.colors,
    ink: {
      50: '#f5f5f5',
      100: '#e8e8e8',
      200: '#d4d4d4',
      300: '#b0b0b0',
      400: '#8c8c8c',
      500: '#6e6e6e',
      600: '#555555',
      700: '#3d3d3d',
      800: '#222222',
      900: '#000000',
    },
  },
  styles: {
    global: {
      'html, body, #root': {
        minHeight: '100%',
      },
      body: {
        margin: 0,
        bg: '#ffffff',
        color: 'ink.900',
        fontStyle: 'normal',
        fontWeight: 400,
        letterSpacing: '-0.01em',
        fontSize: '1.125rem',
        lineHeight: 1.75,
      },
      '*::placeholder': {
        color: 'ink.400',
      },
    },
  },
  components: {
    Button: {
      baseStyle: {
        borderRadius: '2px',
        fontWeight: 600,
        letterSpacing: '-0.01em',
      },
      variants: {
        solid: {
          bg: 'ink.900',
          color: 'white',
          _hover: {
            bg: 'ink.800',
          },
          _disabled: {
            bg: 'ink.400',
          },
        },
        subtle: {
          bg: 'transparent',
          borderWidth: '1px',
          borderColor: 'ink.300',
          color: 'ink.900',
          _hover: {
            bg: 'transparent',
            borderColor: 'ink.700',
          },
        },
      },
    },
    Heading: {
      baseStyle: {
        fontWeight: 600,
        letterSpacing: '-0.02em',
      },
    },
    Textarea: {
      variants: {
        outline: {
          borderWidth: '1px',
          borderColor: 'ink.200',
          borderRadius: 0,
          bg: 'transparent',
          _hover: {
            borderColor: 'ink.400',
          },
          _focusVisible: {
            borderColor: 'ink.700',
            boxShadow: 'none',
          },
        },
      },
    },
  },
})
