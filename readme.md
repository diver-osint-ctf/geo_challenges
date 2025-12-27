# CTFd Geo Challenges Plugin

A geographic challenges plugin for CTFd that allows challenge creators to set location-based puzzles. Players must find specific geographic coordinates to solve challenges.

## Overview

This plugin was originally developed for [Oscar Zulu](https://oscarzulu.org) to enhance Capture The Flag competitions with geographic elements, and is now being offered to the broader CTF creator community.

## Features

- Create challenges requiring users to find specific geographic locations
- Set custom tolerance radius for accepting answers
- Geocoding support for location search
- Multilingual interface (English, French, Spanish)
- Interactive map interface using Leaflet and OpenStreetMap
- Coordinate selection via map click or search
- Visual feedback showing tolerance zones
- Mobile-friendly responsive design

## Installation

1. Clone this repository to your CTFd installation's `plugins/` directory:
   ```bash
   cd /path/to/CTFd/plugins
   git clone https://github.com/yourusername/geo_challenges.git
   ```

2. Restart your CTFd instance to load the plugin.

## Usage

### Creating a Geo Challenge

* example chall -> https://github.com/diver-osint-ctf/geomap_sample/blob/main/README.md

1. In the CTFd admin panel, go to Challenges → Create Challenge
2. Select "geo" as the challenge type
3. Fill in the standard fields (name, category, description, etc.)
4. Set the point value and select the target location on the map
5. Set a tolerance radius (in meters)
6. Save your challenge

### Solving a Geo Challenge

Players will:
1. See an interactive map when viewing the challenge
2. Place a marker by clicking on the map or using the search box
3. Submit their answer
4. Receive points if their selected location is within the tolerance radius of the target
