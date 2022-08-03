import os
import requests
import json
import pandas as pd
import math

class Utils:

    api = {
        'transactions': 'https://draft.premierleague.com/api/draft/league/36298/transactions',
        'elements': 'https://draft.premierleague.com/api/bootstrap-static',
        'details': 'https://draft.premierleague.com/api/league/36298/details',
        'element_status': 'https://draft.premierleague.com/api/league/36298/element-status'
    }

    def __init__(self):
        self.session = requests.session()
        url = 'https://users.premierleague.com/accounts/login/'
        payload = {
        'password': os.environ['pwd'],
        'login': os.environ['email'],
        'redirect_uri': 'https://fantasy.premierleague.com/a/login',
        'app': 'plfpl-web'
        }
        self.session.post(url, data=payload)

    def get_data(self, df_name):
        # Dataframes from the details.json
        if df_name == 'league_entries':
            r = self.session.get(self.api['details']).json()
            league_entry_df = pd.json_normalize(r['league_entries'])
                
            return league_entry_df
        
        elif df_name == 'matches':
            r = self.session.get(self.api['details']).json()
            matches_df = pd.json_normalize(r['matches'])
                
            return matches_df
        
        elif df_name == 'standings':
            r = self.session.get(self.api['details']).json()
            standings_df = pd.json_normalize(r['standings'])
                
            return standings_df
        
        # Dataframes from the elements.json
        elif df_name == 'elements':
            r = self.session.get(self.api['elements']).json()
            elements_df = pd.json_normalize(r['elements'])
                
            return elements_df
        
        elif df_name == 'element_types':
            r = self.session.get(self.api['elements']).json()
            element_types_df = pd.json_normalize(r['element_types'])
                
            return element_types_df
        
        # Dataframes from the transactions.json
        elif df_name == 'transactions':
            r = self.session.get(self.api['transactions']).json()
            transactions_df = pd.json_normalize(r['transactions'])
                
            return transactions_df
        
        # Dataframes from the element_status.json
        elif df_name == 'element_status':
            r = self.session.get(self.api['element_status']).json()
            element_status_df = pd.json_normalize(r['element_status'])
                
            return element_status_df

    def get_team_players(self):
        
        # Pull the required dataframes
        element_status_df = self.get_data('element_status')
        elements_df = self.get_data('elements')
        element_types_df = self.get_data('element_types')
        league_entry_df = self.get_data('league_entries')
        standings_df = self.get_data('standings')
        
        # Built the initial player -> team dataframe
        players_df = (pd.merge(element_status_df,
                            league_entry_df,
                            how="outer",
                            left_on='owner',
                            right_on='entry_id'
                            )
                .drop(columns=['in_accepted_trade',
                                'owner',
                                'status',
                                'entry_id',
                                'entry_name',
                                'id',
                                'joined_time',
                                'player_last_name',
                                'short_name',
                                'waiver_pick'])
                .rename(columns={'player_first_name':'team'})
                )
        
        # Get the element details
        players_df = pd.merge(players_df, elements_df, how="outer", left_on='element', right_on='id')
        players_df = players_df[['team_x',
                                'element',
                                'web_name',
                                'first_name',
                                'second_name',
                                'total_points',
                                'goals_scored',
                                'goals_conceded',
                                'clean_sheets',
                                'assists',
                                'bonus',
                                'draft_rank',
                                'element_type',
                                'points_per_game',
                                'red_cards',
                                'yellow_cards'
                                ]]
        
        # Get the player types (GK, FWD etc.)
        players_df = (pd.merge(players_df,
                            element_types_df,
                            how='outer',
                            left_on='element_type',
                            right_on='id')
                    .drop(columns=['id',
                                    'plural_name_short',
                                    'singular_name',
                                    'singular_name_short'])
                    )

        return players_df

    def get_fixtures(self):
        matches_df = self.get_data('matches')
        league_entry_df = self.get_data('league_entries')

        # Join to get team names and player names of entry 1 (home team)
        matches_df = pd.merge(matches_df,
                            league_entry_df[['id', 'player_first_name']],
                            how='left',
                            left_on='league_entry_1',
                            right_on='id')

        # Join to get team names and player names of entry 2 (away team)
        matches_df = pd.merge(matches_df,
                            league_entry_df[['id', 'player_first_name']],
                            how='left',
                            left_on='league_entry_2',
                            right_on='id')

        # Drop unused columns, rename for clearer columns
        matches_df = (matches_df
                    .drop(['finished', 'started', 'id_x', 'id_y', 'league_entry_1', 'league_entry_2'], axis=1)
                    .rename(columns={'event':'match',
                            'player_first_name_x': 'home_player',
                            'league_entry_1_points': 'home_score',
                            'player_first_name_y': 'away_player',
                            'league_entry_2_points': 'away_score',
                            })
                    )

        return matches_df

    def current_gw(self):
        matches_df = self.get_data('matches')       
        num_gameweeks = matches_df[matches_df['finished'] == True]['event'].max()
        
        if math.isnan(num_gameweeks):
            num_gameweeks = 1

        return num_gameweeks