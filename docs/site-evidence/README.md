# Appicon site accessibility evidence

The Pages landing page was rendered from `site/` and inspected at these viewports:

| Viewport | Evidence | Layout result |
| --- | --- | --- |
| 1440 × 1000 | [Desktop](desktop.png) | Two-column hero, visible primary navigation, four-column proof strip |
| 390 × 844 | [Mobile install guide](mobile.png) | Operating-system choices and install cards reflow without horizontal page overflow |
| 1440 × 1000 | [Forced colors](forced-colors.png) | Navigation, code, buttons, borders, and focus affordances remain distinct in Chromium high-contrast mode |
| 720 × 500 at 2× scale | [200% scale](zoom-200.png) | Help content remains readable without clipped text or overlapping controls |
| 1440 × 6000 | [Full desktop guide](guide-desktop.png) | Installation choices, first-use steps, task guides, help, and technical handoff |
| 390 × 6000 | [Full mobile guide](guide-mobile.png) | User guide reflows into one readable column with scrollable commands |

Lighthouse 12.8.2 reported an accessibility score of **100/100** with no failed accessibility audits on August 31, 2026. The first run exposed overlapping copy-button targets on the landing page. Making every code block its positioning container resolved the failure.

The layout also provides:

- a keyboard-visible skip link and focus indicator;
- keyboard-operable copy buttons and operating-system choices with live status updates;
- semantic header, navigation, main, section, article, list, and footer landmarks;
- text alternatives for decorative marks and a text description for the terminal example;
- reflow breakpoints at 850 and 480 CSS pixels;
- a reduced-motion mode that disables smooth scrolling and animation;
- light and dark palettes with contrast validated in the rendered page.
- forced-color rules that preserve control boundaries and selected states.
