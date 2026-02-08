import { extendTheme, theme as baseTheme, type ThemeConfig } from '@chakra-ui/react'

const config: ThemeConfig = {
  initialColorMode: 'light',
  useSystemColorMode: false,
}

const fonts = {
  heading: `'Palatino Linotype', 'Book Antiqua', 'Times New Roman', serif`,
  body: `'Iowan Old Style', 'Palatino Linotype', 'Book Antiqua', Georgia, serif`,
}

export const theme = extendTheme({
  config,
  fonts,
  colors: {
    ...baseTheme.colors,
    ink: {
      50: '#f8f8f8',
      100: '#ededed',
      200: '#dbdbdb',
      300: '#c4c4c4',
      400: '#adadad',
      500: '#8d8d8d',
      600: '#666666',
      700: '#4a4a4a',
      800: '#2e2e2e',
      900: '#151515',
    },
    pastel: {
      50: '#fbfdff',
      100: '#f4f9ff',
      200: '#ecf6ff',
      300: '#e6f0ff',
      400: '#e9f8f2',
      500: '#fef6e8',
      600: '#fceff3',
      700: '#f3ebff',
      800: '#eef7ff',
      900: '#f6f3ff',
    },
  },
  styles: {
    global: {
      'html, body, #root': {
        minHeight: '100%',
      },
      body: {
        margin: 0,
        bg: '#d0bcb0',
        color: 'ink.900',
        fontStyle: 'italic',
        fontWeight: 500,
        letterSpacing: '0.01em',
      },
      '*::placeholder': {
        color: 'ink.500',
      },
    },
  },
  components: {
    Button: {
      baseStyle: {
        borderRadius: '4px',
        fontWeight: 700,
        fontStyle: 'italic',
        letterSpacing: '0.01em',
      },
      variants: {
        solid: {
          bg: 'ink.900',
          color: 'white',
          _hover: {
            bg: 'black',
          },
          _disabled: {
            bg: 'ink.500',
          },
        },
        subtle: {
          bg: 'transparent',
          borderWidth: '1px',
          borderColor: 'ink.600',
          color: 'ink.900',
          _hover: {
            bg: 'transparent',
            borderColor: 'ink.800',
          },
        },
      },
    },
    Heading: {
      baseStyle: {
        fontStyle: 'italic',
        fontWeight: 700,
        letterSpacing: '0.02em',
      },
    },
    Textarea: {
      variants: {
        outline: {
          borderColor: 'ink.600',
          bg: 'transparent',
          fontStyle: 'italic',
          _hover: {
            borderColor: 'ink.800',
          },
          _focusVisible: {
            borderColor: 'black',
            boxShadow: '0 0 0 1px var(--chakra-colors-black)',
          },
        },
      },
    },
  },
})
