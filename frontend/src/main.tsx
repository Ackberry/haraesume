import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { ChakraProvider } from '@chakra-ui/react'
import { Auth0Provider } from '@auth0/auth0-react'
import App from './App.tsx'
import { theme } from './theme'

const domain = import.meta.env.VITE_AUTH0_DOMAIN?.trim()
const clientId = import.meta.env.VITE_AUTH0_CLIENT_ID?.trim()
const audience = import.meta.env.VITE_AUTH0_AUDIENCE?.trim()
const redirectUri = import.meta.env.VITE_AUTH0_REDIRECT_URI?.trim() || window.location.origin

if (!domain || !clientId) {
  throw new Error('Missing Auth0 config. Set VITE_AUTH0_DOMAIN and VITE_AUTH0_CLIENT_ID.')
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Auth0Provider
      domain={domain}
      clientId={clientId}
      authorizationParams={{
        redirect_uri: redirectUri,
        ...(audience ? { audience } : {}),
      }}
    >
      <ChakraProvider theme={theme}>
        <App />
      </ChakraProvider>
    </Auth0Provider>
  </StrictMode>,
)
