# FEATURE PLAN - ACTIVITY OVERVIEW
This file is a feature plan for planned functionality.
As a reminder, the purpose of this entire project is to
be a project-based tutorial in learning how to write idiomatic Go.
Therefore, your (AI agent's) role is to create well-designed tutorials
and answer specific questions when asked. I am here to learn how to code,
but I do not need you to make me look things up myself.

## FEATURE - ACTIVITY OVERVIEW PAGE
- Create a page, at route `/activity`, which displays multiple past screenshots
in a gallery view with thumbnails
- At a minimum, we will be able to see past (automated and manual) screenshots:
  - Each automatic screenshot will be taken at semi-random time intervals;
    - One per hour of the day, but at a random time; examples:
      - Correct: 00:00, 01:24, 02:37, 03:01, 04:57, etc.
      - Incorrect: 00:00, 01:00, 02:00, 03:00, 04:00, etc.
- This page will also display manual screenshots, chronologically
in addition to the automatic screenshots
- This page will only show the last 24 automatic or manual screenshots
- For the time being, the server should keep all screenshots which are not
older than 1 week (delete those that are 1 week old or older)
- Stylesheets, UI elements, etc. should be kept to a minimum
- Backend is Go, exclusively
- No option to delete screenshots exposed to user

