import os
import requests
import json
import pandas as pd
import math
import unidecode


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
        'password': os.getenv('PWD'),
        'login': os.getenv('EMAIL'),
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

        elif df_name == 'teams':
            r = self.session.get(self.api['elements']).json()
            elements_df = pd.json_normalize(r['teams'])
                
            return elements_df
        
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
        teams_df = self.get_data('teams')
        standings_df = self.get_data('standings')
        
        # Built the initial player -> team dataframe
        players_df = (pd.merge(element_status_df,
                               league_entry_df,
                               how="outer",
                               left_on='owner',
                               right_on='entry_id')
                .drop(columns=[
                    'in_accepted_trade',
                    'owner',
                    'status',
                    'entry_id',
                    'entry_name',
                    'id',
                    'joined_time',
                    'player_last_name',
                    'short_name',
                    'waiver_pick'])
                .rename(columns={'player_first_name':'team_x'})
        )
        
        # Get the element details
        players_df = pd.merge(players_df, elements_df, how="outer", left_on='element', right_on='id')
        players_df = players_df[[
            'team_x',
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
            'yellow_cards',
            'team'
        ]]
        
        # Get the player types (GK, FWD etc.)
        players_df = (pd.merge(players_df,
                               element_types_df,
                               how='outer',
                               left_on='element_type',
                               right_on='id')
                    .drop(columns=[
                        'id',
                        'plural_name_short',
                        'singular_name',
                        'singular_name_short'])
        )

        players_df = (pd.merge(players_df, 
                               teams_df,
                               how='outer',
                               left_on='team',
                               right_on='id')
                        .drop(
                            columns=[
                            'code',
                            'id',
                            'pulse_id'
                        ])
                        .rename(columns={'name':'team_name'}))

        players_df['web_name'] = players_df['web_name'].apply(self.remove_accents)

        return players_df

    def get_player_attr(self, player: str, attr: str):
        players_df = self.get_team_players()

        if attr not in players_df.columns:
            raise ValueError(f'Attribrute \'{attr}\' is not available for players')

        selected_players_df = players_df.loc[players_df['web_name'].str.lower() == player.lower()]
        selected_players_df = selected_players_df[[
            'first_name',
            'second_name',
            attr
        ]]
        return selected_players_df

    def get_team(self, owner: str, pos: str=None):
        players_df = self.get_team_players()
        team_df = players_df.loc[players_df['team_x'].str.lower() == owner.lower()]
        if pos is not None:
            team_df = team_df.loc[players_df['plural_name'].str.lower() == pos.lower()]

        if team_df.empty:
            raise ValueError(f'No players owned by: {owner}')

        return team_df

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
        num_gameweeks += 1
        
        if math.isnan(num_gameweeks):
            num_gameweeks = 1

        return num_gameweeks

    def remove_accents(self, string: str):
        return unidecode.unidecode(string)