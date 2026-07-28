import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://alfian57.github.io',
  base: '/gin-kit',
  integrations: [
    starlight({
      title: 'gin-kit',
      description: 'Everything included, nothing hidden — an opinionated Go framework built on Gin.',
      logo: {
        light: './src/assets/gin-kit-gopher-light.png',
        dark: './src/assets/gin-kit-gopher-dark.png',
        alt: 'gin-kit gopher mascot',
      },
      favicon: '/favicon.png',
      head: [
        {
          tag: 'meta',
          attrs: {
            property: 'og:image',
            content: 'https://alfian57.github.io/gin-kit/og-image.png',
          },
        },
        {
          tag: 'meta',
          attrs: {
            name: 'twitter:image',
            content: 'https://alfian57.github.io/gin-kit/og-image.png',
          },
        },
      ],
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/Alfian57/gin-kit',
        },
      ],
      customCss: ['./src/styles/custom.css'],
      editLink: {
        baseUrl: 'https://github.com/Alfian57/gin-kit/edit/main/website/',
      },
      sidebar: [
        {
          label: 'Start',
          items: [
            { label: 'Introduction', slug: 'index' },
            { label: 'Quickstart', slug: 'getting-started' },
            { label: 'Project types', slug: 'project-types' },
          ],
        },
        {
          label: 'Core concepts',
          items: [
            { label: 'Architecture', slug: 'architecture' },
            { label: 'Responses and validation', slug: 'responses-validation' },
            { label: 'Request and response DTOs', slug: 'dto' },
            { label: 'Validation', slug: 'validation' },
            { label: 'Filtering and pagination', slug: 'querying' },
            { label: 'Middleware and errors', slug: 'error-handling' },
            { label: 'Configuration', slug: 'configuration' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Database and ORM', slug: 'database-orm' },
            { label: 'Seeding and factories', slug: 'seeding-factories' },
            { label: 'Authentication and security', slug: 'auth-security' },
            { label: 'Authorization', slug: 'authorization' },
            { label: 'Queues and scheduling', slug: 'background-work' },
            { label: 'Caching and events', slug: 'caching-events' },
            { label: 'Realtime updates', slug: 'realtime' },
            { label: 'Mail and file storage', slug: 'mail-storage' },
            { label: 'API documentation', slug: 'api-docs' },
            { label: 'Observability', slug: 'observability' },
            { label: 'Devtools dashboard', slug: 'devtools' },
            { label: 'CLI and generators', slug: 'cli-generators' },
            { label: 'Customizing the runtime', slug: 'customization' },
            { label: 'UI mode', slug: 'ui-mode' },
            { label: 'Sessions and CSRF', slug: 'sessions' },
            { label: 'Testing', slug: 'testing' },
            { label: 'Deployment', slug: 'deployment' },
          ],
        },
        {
          label: 'Project',
          items: [
            { label: 'AI agents', slug: 'ai-agents' },
            { label: 'Contributing', slug: 'contributing' },
            { label: 'Upgrade notes', slug: 'upgrading' },
            { label: 'Releasing', slug: 'releasing' },
            {
              label: 'Changelog',
              link: 'https://github.com/Alfian57/gin-kit/blob/main/CHANGELOG.md',
              attrs: { target: '_blank' },
            },
          ],
        },
      ],
    }),
  ],
});
