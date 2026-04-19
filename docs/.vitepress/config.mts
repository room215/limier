import { defineConfig } from 'vitepress'

const repo = 'https://github.com/room215/limier'

export default defineConfig({
  lang: 'en-US',
  title: 'Limier',
  titleTemplate: ':title | Limier',
  description:
    'Fixture-based dependency behavior review for CI and local security analysis.',
  // GitHub Pages serves this repository at /limier/.
  // Change this to / if you move the docs to a custom domain or user site.
  base: '/limier/',
  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Reference', link: '/reference/cli' },
      { text: 'Project Notes', link: '/launch-readiness' }
    ],
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Overview', link: '/' },
          { text: 'Getting Started', link: '/guide/getting-started' },
          { text: 'Review Your Own Project', link: '/guide/review-your-own-project' },
          { text: 'Understand Results', link: '/guide/understand-results' },
          { text: 'Use In CI', link: '/guide/ci-and-deploy' }
        ]
      },
      {
        text: 'Reference',
        items: [
          { text: 'CLI', link: '/reference/cli' },
          { text: 'Scenario File', link: '/reference/scenario-file' },
          { text: 'Rules File', link: '/reference/rules-file' }
        ]
      },
      {
        text: 'Project Notes',
        items: [
          { text: 'Launch Readiness', link: '/launch-readiness' },
          {
            text: 'Product Development Plan',
            link: '/product-development-plan-dependency-behavior-diff-limier-v4'
          },
          {
            text: 'Business Plan',
            link: '/business-plan-dependency-behavior-diff-limier-v3'
          }
        ]
      }
    ],
    search: {
      provider: 'local'
    },
    socialLinks: [{ icon: 'github', link: repo }],
    editLink: {
      pattern: 'https://github.com/room215/limier/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    },
    outline: {
      level: [2, 3]
    },
    footer: {
      message: 'Built with VitePress.',
      copyright: 'Copyright © 2026 room215'
    }
  }
})
