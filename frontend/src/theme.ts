import { extendTheme, theme as baseTheme, type ThemeConfig } from '@chakra-ui/react'

const config: ThemeConfig = {
  initialColorMode: 'light',
  useSystemColorMode: false,
}

const fonts = {
  heading: `'Space Grotesk', 'Avenir Next', 'Trebuchet MS', sans-serif`,
  body: `'Plus Jakarta Sans', 'Avenir Next', 'Trebuchet MS', sans-serif`,
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
      },
      '*::placeholder': {
        color: 'ink.500',
      },
    },
  },
  components: {
    Button: {
      baseStyle: {
        borderRadius: '12px',
        fontWeight: 600,
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
          borderColor: 'ink.300',
          color: 'ink.900',
          _hover: {
            bg: 'transparent',
            borderColor: 'ink.500',
          },
        },
      },
    },
    Textarea: {
      variants: {
        outline: {
          borderColor: 'ink.300',
          bg: 'transparent',
          _hover: {
            borderColor: 'ink.500',
          },
          _focusVisible: {
            borderColor: 'ink.800',
            boxShadow: '0 0 0 1px var(--chakra-colors-black)',
          },
        },
      },
    },
  },
})
